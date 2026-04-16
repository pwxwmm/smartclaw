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
}

func AllAgentTypes() []DomainAgentType {
	return []DomainAgentType{AgentNetwork, AgentDatabase, AgentInfra, AgentApp, AgentSecurity}
}

func GetAgent(agentType DomainAgentType) (DomainAgent, bool) {
	a, ok := BuiltInAgents[agentType]
	return a, ok
}
