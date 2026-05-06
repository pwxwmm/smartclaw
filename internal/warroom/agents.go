package warroom

type DomainAgent struct {
	Type               DomainAgentType
	Name               string
	Description        string
	InvestigationSteps []string
	Tools              []string
	FocusAreas         []string
}

var BuiltInAgents = map[DomainAgentType]DomainAgent{
	AgentNetwork: {
		Type:        AgentNetwork,
		Name:        "Network Investigator",
		Description: "Diagnoses network connectivity, DNS, load balancer, and routing issues",
		InvestigationSteps: []string{
			"Check DNS resolution for affected services",
			"Verify network connectivity between services",
			"Review load balancer health checks and configs",
			"Analyze firewall and security group rules",
			"Check for network partition or split-brain",
		},
		Tools:      []string{"bash", "read_file", "grep", "web_fetch"},
		FocusAreas: []string{"DNS", "connectivity", "latency", "packet_loss", "firewall", "load_balancer", "TLS"},
	},
	AgentDatabase: {
		Type:        AgentDatabase,
		Name:        "Database Investigator",
		Description: "Diagnoses database performance, replication, and data integrity issues",
		InvestigationSteps: []string{
			"Check database connectivity and response times",
			"Review replication lag and status",
			"Analyze slow queries and query plans",
			"Check disk space and I/O metrics",
			"Verify connection pool utilization",
		},
		Tools:      []string{"bash", "read_file", "grep"},
		FocusAreas: []string{"replication_lag", "slow_queries", "connections", "disk_io", "deadlocks", "locks"},
	},
	AgentInfra: {
		Type:        AgentInfra,
		Name:        "Infrastructure Investigator",
		Description: "Diagnoses Kubernetes, container, and host-level infrastructure issues",
		InvestigationSteps: []string{
			"Check pod/container status and events",
			"Review resource utilization (CPU, memory, disk)",
			"Check node health and conditions",
			"Verify configuration and environment variables",
			"Review recent deployments and changes",
		},
		Tools:      []string{"bash", "read_file", "grep", "glob"},
		FocusAreas: []string{"OOMKilled", "CrashLoopBackOff", "resource_limits", "node_pressure", "deployment", "config"},
	},
	AgentApp: {
		Type:        AgentApp,
		Name:        "Application Investigator",
		Description: "Diagnoses application-level errors, performance, and logic issues",
		InvestigationSteps: []string{
			"Check application error rates and types",
			"Review application logs for exceptions",
			"Analyze request traces and latency distribution",
			"Check dependency health and circuit breakers",
			"Review recent code changes and deployments",
		},
		Tools:      []string{"bash", "read_file", "grep", "glob", "code_search"},
		FocusAreas: []string{"errors", "exceptions", "latency", "timeouts", "circuit_breakers", "memory_leaks"},
	},
	AgentSecurity: {
		Type:        AgentSecurity,
		Name:        "Security Investigator",
		Description: "Diagnoses security incidents, unauthorized access, and compliance issues",
		InvestigationSteps: []string{
			"Review authentication and authorization logs",
			"Check for unauthorized access attempts",
			"Verify security group and network policy compliance",
			"Review certificate expiration and TLS config",
			"Check for anomalous traffic patterns",
		},
		Tools:      []string{"bash", "read_file", "grep", "web_fetch"},
		FocusAreas: []string{"auth_failures", "unauthorized_access", "certificates", "compliance", "anomalies"},
	},
	AgentReasoning: {
		Type:        AgentReasoning,
		Name:        "Reasoning Investigator",
		Description: "Performs causal reasoning, hypothesis validation, and cross-domain correlation analysis to identify root causes",
		InvestigationSteps: []string{
			"Collect and correlate findings from other domain agents",
			"Build causal chains linking symptoms to potential root causes",
			"Form and test hypotheses against available evidence",
			"Identify contradictions or gaps in current understanding",
			"Rank root cause hypotheses by likelihood and propose verification steps",
		},
		Tools:      []string{"bash", "read_file", "grep", "web_fetch"},
		FocusAreas: []string{"causal_analysis", "hypothesis_testing", "correlation", "root_cause_ranking", "evidence_synthesis", "contradiction_detection"},
	},
	AgentTraining: {
		Type:        AgentTraining,
		Name:        "Distributed Training Investigator",
		Description: "Diagnoses multi-node multi-GPU training failures including NCCL, CUDA, checkpoint, and parallelism issues",
		InvestigationSteps: []string{
			"Check GPU health, CUDA version, and driver compatibility across nodes",
			"Review NCCL and distributed communication logs for timeouts and errors",
			"Analyze network topology and RDMA/RoCE configuration for training traffic",
			"Inspect checkpoint integrity, save/load paths, and recovery state",
			"Verify data/tensor/pipeline parallelism config and rank assignment",
			"Check for OOM, gradient overflow, loss divergence, and learning rate issues",
		},
		Tools:      []string{"bash", "read_file", "grep", "glob"},
		FocusAreas: []string{"NCCL_timeout", "CUDA_error", "GPU_health", "RDMA_RoCE", "checkpoint_corruption", "OOM", "gradient_overflow", "loss_divergence", "tensor_parallelism", "data_parallelism", "pipeline_parallelism", "rank_mismatch", "distributed_checkpoint"},
	},
	AgentInference: {
		Type:        AgentInference,
		Name:        "AI Inference Investigator",
		Description: "Diagnoses AI model serving issues in vLLM, SGLang, and similar inference frameworks including scheduling, KV cache, and serving errors",
		InvestigationSteps: []string{
			"Check inference server health, startup logs, and model loading status",
			"Review KV cache utilization, memory allocation, and eviction patterns",
			"Analyze request scheduling, queuing delays, and batching behavior",
			"Inspect GPU memory fragmentation and CUDA context errors",
			"Verify model config, tokenizer loading, and dtype/quantization settings",
			"Check for timeout, OOM, token limit, and context length errors",
			"Review API endpoint health, connection pooling, and load balancer config",
		},
		Tools:      []string{"bash", "read_file", "grep", "glob", "web_fetch"},
		FocusAreas: []string{"KV_cache_pressure", "model_loading_failure", "OOM", "scheduling_timeout", "batch_queue_overflow", "CUDA_context_error", "token_limit_exceeded", "context_length_overflow", "dtype_mismatch", "quantization_error", "tokenizer_failure", "vLLM", "SGLang", "continuous_batching", "prefix_caching", "speculative_decoding"},
	},
}

func AllAgentTypes() []DomainAgentType {
	return []DomainAgentType{AgentNetwork, AgentDatabase, AgentInfra, AgentApp, AgentSecurity, AgentReasoning, AgentTraining, AgentInference}
}

func GetAgent(agentType DomainAgentType) (DomainAgent, bool) {
	a, ok := BuiltInAgents[agentType]
	return a, ok
}
