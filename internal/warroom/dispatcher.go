package warroom

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type DispatchPlan struct {
	Phase1Agents []DomainAgentType
	Phase2Agent  DomainAgentType
	Phase3Pool   []DomainAgentType
}

type Dispatcher struct {
	runner *LLMAgentRunner
	mu     sync.Mutex
	plans  map[string]*DispatchPlan
}

func NewDispatcher(runner *LLMAgentRunner) *Dispatcher {
	return &Dispatcher{
		runner: runner,
		plans:  make(map[string]*DispatchPlan),
	}
}

func SelectAgents(description string) DispatchPlan {
	desc := strings.ToLower(description)

	var phase1 []DomainAgentType
	var phase3Pool []DomainAgentType

	trainingScore := keywordScore(desc, []string{"训练", "training", "gpu", "cuda", "nccl", "oom", "多卡", "多机", "分布式训练", "loss", "梯度", "checkpoint", "pytorch", "megatron", "deepspeed"})
	inferenceScore := keywordScore(desc, []string{"推理", "inference", "vllm", "v-llm", "sglang", "serving", "served", "模型服务", "推理服务", "502", "503", "latency", "延迟", "吞吐", "throughput", "kv cache", "批处理", "batching"})
	networkScore := keywordScore(desc, []string{"网络", "network", "dns", "延迟", "latency", "丢包", "packet", "lb", "负载均衡", "防火墙", "firewall", "连接超时", "timeout", "连接拒绝", "connection refused"})
	dbScore := keywordScore(desc, []string{"数据库", "database", "db", "mysql", "postgres", "redis", "mongo", "慢查询", "复制", "replication", "死锁", "deadlock", "连接池"})
	infraScore := keywordScore(desc, []string{"节点", "node", "pod", "容器", "container", "k8s", "kubernetes", "cpu", "内存", "磁盘", "disk", "oom", "crashloop", "部署", "deploy", "infra", "基础设施"})
	appScore := keywordScore(desc, []string{"应用", "app", "应用层", "错误率", "error rate", "日志", "log", "异常", "exception", "5xx", "4xx", "超时", "timeout", "熔断", "circuit breaker"})
	securityScore := keywordScore(desc, []string{"安全", "security", "攻击", "attack", "未授权", "unauthorized", "证书", "certificate", "tls", "ssl", "合规", "compliance", "泄露", "breach"})

	type scoreEntry struct {
		agentType DomainAgentType
		score     int
	}
	scores := []scoreEntry{
		{AgentTraining, trainingScore},
		{AgentInference, inferenceScore},
		{AgentNetwork, networkScore},
		{AgentDatabase, dbScore},
		{AgentInfra, infraScore},
		{AgentApp, appScore},
		{AgentSecurity, securityScore},
	}

	for _, s := range scores {
		if s.score >= 2 {
			phase1 = append(phase1, s.agentType)
		} else if s.score == 1 {
			phase3Pool = append(phase3Pool, s.agentType)
		}
	}

	if len(phase1) == 0 {
		if infraScore > 0 || appScore > 0 {
			phase1 = []DomainAgentType{AgentInfra, AgentApp}
		} else {
			phase1 = []DomainAgentType{AgentInfra}
		}
	}

	if len(phase1) > 4 {
		phase1 = phase1[:4]
	}

	return DispatchPlan{
		Phase1Agents: phase1,
		Phase2Agent:  AgentReasoning,
		Phase3Pool:   phase3Pool,
	}
}

func keywordScore(text string, keywords []string) int {
	score := 0
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			score++
		}
	}
	return score
}

type StagedExecutor struct {
	dispatcher *Dispatcher
	coordinator *WarRoomCoordinator
}

func NewStagedExecutor(dispatcher *Dispatcher, coordinator *WarRoomCoordinator) *StagedExecutor {
	return &StagedExecutor{
		dispatcher: dispatcher,
		coordinator: coordinator,
	}
}

func (e *StagedExecutor) ExecuteStaged(ctx context.Context, sessionID string, description string) error {
	plan := SelectAgents(description)

	e.dispatcher.mu.Lock()
	e.dispatcher.plans[sessionID] = &plan
	e.dispatcher.mu.Unlock()

	e.addTimelineEntry(sessionID, "", "dispatch_plan",
		fmt.Sprintf("Phase 1: %v | Phase 2: %s | Phase 3 pool: %v",
			agentTypeNames(plan.Phase1Agents), plan.Phase2Agent, agentTypeNames(plan.Phase3Pool)))

	var wg sync.WaitGroup
	var phase1Findings []Finding
	var findingsMu sync.Mutex

	for _, agentType := range plan.Phase1Agents {
		agent, ok := BuiltInAgents[agentType]
		if !ok {
			continue
		}

		wg.Add(1)
		go func(at DomainAgentType, a DomainAgent) {
			defer wg.Done()

			e.updateAgentStatus(sessionID, at, AgentStatusRunning)

			taskPrompt := fmt.Sprintf("Investigate: %s\n\nInvestigation Steps:\n%s\n\nFocus: %s",
				description,
				strings.Join(a.InvestigationSteps, "\n"),
				strings.Join(a.FocusAreas, ", "))

			result, err := e.dispatcher.runner.RunAgent(ctx, at, taskPrompt, a.Tools)

			if err != nil {
				slog.Error("warroom: phase 1 agent failed", "agent", at, "error", err)
				e.updateAgentStatus(sessionID, at, AgentStatusFailed)
				return
			}

			e.updateAgentStatus(sessionID, at, AgentStatusComplete)

			if result != "" {
				finding := Finding{
					ID:          uuid.New().String(),
					AgentType:   at,
					Category:    "symptom",
					Title:       fmt.Sprintf("%s finding", a.Name),
					Description: truncateString(result, 500),
					Confidence:  0.6,
					Evidence:    []string{truncateString(result, 200)},
					CreatedAt:   time.Now(),
				}

				findingsMu.Lock()
				phase1Findings = append(phase1Findings, finding)
				findingsMu.Unlock()

				if err := e.coordinator.SubmitFinding(sessionID, at, finding); err != nil {
					slog.Error("warroom: failed to submit finding", "agent", at, "error", err)
				}
			}
		}(agentType, agent)
	}

	wg.Wait()

	e.addTimelineEntry(sessionID, "", "phase1_complete",
		fmt.Sprintf("Phase 1 complete: %d findings from %d agents", len(phase1Findings), len(plan.Phase1Agents)))

	if ctx.Err() != nil {
		return ctx.Err()
	}

	reasoningAgent, ok := BuiltInAgents[plan.Phase2Agent]
	if !ok {
		return nil
	}

	e.updateAgentStatus(sessionID, plan.Phase2Agent, AgentStatusRunning)
	e.addTimelineEntry(sessionID, plan.Phase2Agent, "phase2_started", "Reasoning agent analyzing Phase 1 findings")

	var findingsSummary strings.Builder
	findingsSummary.WriteString(fmt.Sprintf("Incident: %s\n\n", description))
	findingsSummary.WriteString("Phase 1 Findings:\n\n")
	for i, f := range phase1Findings {
		findingsSummary.WriteString(fmt.Sprintf("### Finding %d (%s - %s)\n%s\n\n", i+1, f.AgentType, f.Title, f.Description))
	}
	findingsSummary.WriteString("\nBased on these findings:\n")
	findingsSummary.WriteString("1. What is the most likely root cause?\n")
	findingsSummary.WriteString("2. What additional investigation is needed?\n")
	findingsSummary.WriteString("3. Are there any correlations between findings from different agents?\n")
	findingsSummary.WriteString("4. What are the recommended next steps?\n")

	reasoningResult, err := e.dispatcher.runner.RunAgent(ctx, plan.Phase2Agent, findingsSummary.String(), reasoningAgent.Tools)

	if err != nil {
		slog.Error("warroom: phase 2 reasoning failed", "error", err)
		e.updateAgentStatus(sessionID, plan.Phase2Agent, AgentStatusFailed)
	} else {
		e.updateAgentStatus(sessionID, plan.Phase2Agent, AgentStatusComplete)

		if reasoningResult != "" {
			rootCauseFinding := Finding{
				ID:          uuid.New().String(),
				AgentType:   plan.Phase2Agent,
				Category:    "root_cause",
				Title:       "Root Cause Analysis",
				Description: truncateString(reasoningResult, 1000),
				Confidence:  0.7,
				Evidence:    []string{truncateString(reasoningResult, 300)},
				CreatedAt:   time.Now(),
			}
			if err := e.coordinator.SubmitFinding(sessionID, plan.Phase2Agent, rootCauseFinding); err != nil {
				slog.Error("warroom: failed to submit reasoning finding", "error", err)
			}
		}
	}

	e.addTimelineEntry(sessionID, "", "phase2_complete", "Reasoning analysis complete")

	return nil
}

func (e *StagedExecutor) updateAgentStatus(sessionID string, agentType DomainAgentType, status AgentStatus) {
	e.coordinator.mu.Lock()
	if s, exists := e.coordinator.sessions[sessionID]; exists {
		for i := range s.Agents {
			if s.Agents[i].AgentType == agentType {
				s.Agents[i].Status = status
				s.Agents[i].LastActive = time.Now()
				break
			}
		}
		s.Timeline = append(s.Timeline, TimelineEntry{
			Timestamp: time.Now(),
			AgentType: agentType,
			Event:     "agent_" + string(status),
			Details:   fmt.Sprintf("%s is now %s", BuiltInAgents[agentType].Name, status),
		})
	}
	e.coordinator.mu.Unlock()
}

func (e *StagedExecutor) addTimelineEntry(sessionID string, agentType DomainAgentType, event string, details string) {
	e.coordinator.mu.Lock()
	if s, exists := e.coordinator.sessions[sessionID]; exists {
		s.Timeline = append(s.Timeline, TimelineEntry{
			Timestamp: time.Now(),
			AgentType: agentType,
			Event:     event,
			Details:   details,
		})
	}
	e.coordinator.mu.Unlock()
}

func agentTypeNames(types []DomainAgentType) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}
