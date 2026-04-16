package fingerprint

import (
	"hash/fnv"
	"strings"
	"time"
)

// IncidentData is the input for fingerprint generation.
type IncidentData struct {
	ID               string
	Title            string
	Severity         string
	Status           string
	Service          string
	StartedAt        time.Time
	MitigatedAt      *time.Time
	ResolvedAt       *time.Time
	AffectedServices []string
	Labels           map[string]string

	BlastRadius         float64
	IsCriticalPath      bool
	DependencyDepth     int
	SLOBurnRate         float64
	ErrorBudgetUsed     float64
	ToolCallCount       int
	InvestigationSteps  int
	RemediationAttempts int
	AutoRemediated      bool
	HumanIntervention   bool
	Escalated           bool
	EscalationCount     int
	HasRunbook          bool
	HasPostmortem       bool
	IsRecurring         bool
	SimilarPastCount    int
	CategoryHints       []string
}

const fingerprintVersion = 1

// GenerateFingerprint creates a 64-dim feature vector from incident data.
func GenerateFingerprint(data IncidentData) IncidentFingerprint {
	temporal := extractTemporalFeatures(data)
	severity := extractSeverityFeatures(data)
	topology := extractTopologyFeatures(data)
	service := extractServiceFeatures(data)
	impact := extractImpactFeatures(data)
	response := extractResponseFeatures(data)
	category := inferCategories(data.Title, data.Labels, data.CategoryHints)
	label := extractLabelFeatures(data)

	features := FeatureMap{
		Temporal: temporal,
		Severity: severity,
		Topology: topology,
		Service:  service,
		Impact:   impact,
		Response: response,
		Category: category,
		Label:    label,
	}

	var vector [VectorSize]float64
	vector[0] = temporal.HourOfDay
	vector[1] = temporal.DayOfWeek
	vector[2] = temporal.IsBusinessHours
	vector[3] = temporal.IsWeekend
	vector[4] = temporal.Duration
	vector[5] = temporal.TimeToTriage
	vector[6] = temporal.TimeToMitigate

	vector[7] = severity.SeverityLevel
	vector[8] = severity.Escalated
	vector[9] = severity.EscalationCount

	vector[10] = topology.AffectedServiceCount
	vector[11] = topology.BlastRadiusScore
	vector[12] = topology.IsCriticalPath
	vector[13] = topology.DependencyDepth

	vector[14] = service.ServiceType
	vector[15] = service.ServiceCount
	vector[16] = service.PrimaryServiceHash

	vector[17] = impact.UserImpactScore
	vector[18] = impact.SLOViolationCount
	vector[19] = impact.ErrorBudgetBurned
	vector[20] = impact.DataLoss

	vector[21] = response.ToolCallCount
	vector[22] = response.InvestigationSteps
	vector[23] = response.RemediationAttempts
	vector[24] = response.AutoRemediated
	vector[25] = response.HumanIntervention
	// vector[26] reserved for future ResponseFeatures expansion

	vector[27] = category.IsNetworkIssue
	vector[28] = category.IsDatabaseIssue
	vector[29] = category.IsInfraIssue
	vector[30] = category.IsAppIssue
	vector[31] = category.IsSecurityIssue
	vector[32] = category.IsConfigIssue
	vector[33] = category.IsDeploymentIssue
	vector[34] = category.IsCapacityIssue

	vector[35] = label.HasRunbook
	vector[36] = label.HasPostmortem
	vector[37] = label.IsRecurring
	vector[38] = label.SimilarPastCount
	// vector[39-63] reserved for future expansion, zeros by default

	return IncidentFingerprint{
		IncidentID:  data.ID,
		Vector:      vector,
		Features:    features,
		GeneratedAt: time.Now().UTC(),
		Version:     fingerprintVersion,
	}
}

func extractTemporalFeatures(data IncidentData) TemporalFeatures {
	hour := float64(data.StartedAt.Hour()) / 23.0
	dayOfWeek := float64(data.StartedAt.Weekday()) / 6.0

	isBusinessHours := 0.0
	if data.StartedAt.Weekday() >= time.Monday && data.StartedAt.Weekday() <= time.Friday {
		if data.StartedAt.Hour() >= 9 && data.StartedAt.Hour() < 17 {
			isBusinessHours = 1.0
		}
	}

	isWeekend := 0.0
	if data.StartedAt.Weekday() == time.Saturday || data.StartedAt.Weekday() == time.Sunday {
		isWeekend = 1.0
	}

	duration := 0.0
	if data.ResolvedAt != nil && !data.ResolvedAt.IsZero() {
		hours := data.ResolvedAt.Sub(data.StartedAt).Hours()
		if hours > 72 {
			hours = 72
		}
		if hours < 0 {
			hours = 0
		}
		duration = hours / 72.0
	} else if data.MitigatedAt != nil && !data.MitigatedAt.IsZero() {
		hours := data.MitigatedAt.Sub(data.StartedAt).Hours()
		if hours > 72 {
			hours = 72
		}
		if hours < 0 {
			hours = 0
		}
		duration = hours / 72.0
	}

	timeToTriage := 0.0
	if data.Labels != nil {
		if v, ok := data.Labels["time_to_triage_min"]; ok {
			var mins float64
			for _, c := range v {
				if c >= '0' && c <= '9' {
					mins = mins*10 + float64(c-'0')
				}
			}
			if mins > 60 {
				mins = 60
			}
			timeToTriage = mins / 60.0
		}
	}

	timeToMitigate := 0.0
	if data.MitigatedAt != nil && !data.MitigatedAt.IsZero() {
		mins := data.MitigatedAt.Sub(data.StartedAt).Minutes()
		if mins > 240 {
			mins = 240
		}
		if mins < 0 {
			mins = 0
		}
		timeToMitigate = mins / 240.0
	}

	return TemporalFeatures{
		HourOfDay:       hour,
		DayOfWeek:       dayOfWeek,
		IsBusinessHours: isBusinessHours,
		IsWeekend:       isWeekend,
		Duration:        duration,
		TimeToTriage:    timeToTriage,
		TimeToMitigate:  timeToMitigate,
	}
}

func extractSeverityFeatures(data IncidentData) SeverityFeatures {
	severityLevel := 0.5
	switch strings.ToLower(data.Severity) {
	case "info":
		severityLevel = 0.0
	case "low":
		severityLevel = 0.25
	case "medium":
		severityLevel = 0.5
	case "high":
		severityLevel = 0.75
	case "critical":
		severityLevel = 1.0
	}

	escalated := 0.0
	if data.Escalated {
		escalated = 1.0
	}

	escalationCount := float64(data.EscalationCount) / 5.0
	if escalationCount > 1.0 {
		escalationCount = 1.0
	}

	return SeverityFeatures{
		SeverityLevel:   severityLevel,
		Escalated:       escalated,
		EscalationCount: escalationCount,
	}
}

func extractTopologyFeatures(data IncidentData) TopologyFeatures {
	affectedCount := float64(len(data.AffectedServices)) / 20.0
	if affectedCount > 1.0 {
		affectedCount = 1.0
	}

	blastRadius := data.BlastRadius
	if blastRadius < 0 {
		blastRadius = 0
	}
	if blastRadius > 1 {
		blastRadius = 1
	}

	isCriticalPath := 0.0
	if data.IsCriticalPath {
		isCriticalPath = 1.0
	}

	depDepth := float64(data.DependencyDepth) / 5.0
	if depDepth > 1.0 {
		depDepth = 1.0
	}

	return TopologyFeatures{
		AffectedServiceCount: affectedCount,
		BlastRadiusScore:     blastRadius,
		IsCriticalPath:       isCriticalPath,
		DependencyDepth:      depDepth,
	}
}

func extractServiceFeatures(data IncidentData) ServiceFeatures {
	serviceType := classifyService(data.Service, data.Labels)

	serviceCount := float64(len(data.AffectedServices)+1) / 10.0
	if serviceCount > 1.0 {
		serviceCount = 1.0
	}

	primaryServiceHash := hashServiceName(data.Service)

	return ServiceFeatures{
		ServiceType:        serviceType,
		ServiceCount:       serviceCount,
		PrimaryServiceHash: primaryServiceHash,
	}
}

func extractImpactFeatures(data IncidentData) ImpactFeatures {
	userImpact := data.BlastRadius
	if data.Severity == "critical" {
		userImpact = 1.0
	} else if data.Severity == "high" {
		if userImpact < 0.5 {
			userImpact = 0.5
		}
	}
	if userImpact > 1.0 {
		userImpact = 1.0
	}

	sloViolations := data.SLOBurnRate / 5.0
	if sloViolations > 1.0 {
		sloViolations = 1.0
	}

	errorBudget := data.ErrorBudgetUsed
	if errorBudget < 0 {
		errorBudget = 0
	}
	if errorBudget > 1 {
		errorBudget = 1
	}

	dataLoss := 0.0
	if data.Labels != nil {
		if v, ok := data.Labels["data_loss"]; ok {
			if v == "true" || v == "yes" || v == "1" {
				dataLoss = 1.0
			}
		}
	}

	return ImpactFeatures{
		UserImpactScore:   userImpact,
		SLOViolationCount: sloViolations,
		ErrorBudgetBurned: errorBudget,
		DataLoss:          dataLoss,
	}
}

func extractResponseFeatures(data IncidentData) ResponseFeatures {
	toolCalls := float64(data.ToolCallCount) / 50.0
	if toolCalls > 1.0 {
		toolCalls = 1.0
	}

	investigationSteps := float64(data.InvestigationSteps) / 20.0
	if investigationSteps > 1.0 {
		investigationSteps = 1.0
	}

	remediationAttempts := float64(data.RemediationAttempts) / 5.0
	if remediationAttempts > 1.0 {
		remediationAttempts = 1.0
	}

	autoRemediated := 0.0
	if data.AutoRemediated {
		autoRemediated = 1.0
	}

	humanIntervention := 0.0
	if data.HumanIntervention {
		humanIntervention = 1.0
	}

	return ResponseFeatures{
		ToolCallCount:       toolCalls,
		InvestigationSteps:  investigationSteps,
		RemediationAttempts: remediationAttempts,
		AutoRemediated:      autoRemediated,
		HumanIntervention:   humanIntervention,
	}
}

func extractLabelFeatures(data IncidentData) LabelFeatures {
	hasRunbook := 0.0
	if data.HasRunbook {
		hasRunbook = 1.0
	}

	hasPostmortem := 0.0
	if data.HasPostmortem {
		hasPostmortem = 1.0
	}

	isRecurring := 0.0
	if data.IsRecurring {
		isRecurring = 1.0
	}

	similarPast := float64(data.SimilarPastCount) / 10.0
	if similarPast > 1.0 {
		similarPast = 1.0
	}

	return LabelFeatures{
		HasRunbook:       hasRunbook,
		HasPostmortem:    hasPostmortem,
		IsRecurring:      isRecurring,
		SimilarPastCount: similarPast,
	}
}

// classifyService determines the service type from its name/labels.
func classifyService(service string, labels map[string]string) float64 {
	if labels != nil {
		if t, ok := labels["service_type"]; ok {
			switch strings.ToLower(t) {
			case "frontend", "web", "ui":
				return 0.0
			case "api", "gateway", "bff":
				return 0.25
			case "worker", "job", "consumer", "queue":
				return 0.5
			case "datastore", "database", "cache", "storage":
				return 0.75
			case "infra", "platform", "core", "system":
				return 1.0
			}
		}
	}

	name := strings.ToLower(service)
	switch {
	case strings.Contains(name, "frontend") || strings.Contains(name, "web") || strings.Contains(name, "ui"):
		return 0.0
	case strings.Contains(name, "api") || strings.Contains(name, "gateway") || strings.Contains(name, "bff"):
		return 0.25
	case strings.Contains(name, "worker") || strings.Contains(name, "job") || strings.Contains(name, "consumer") || strings.Contains(name, "queue"):
		return 0.5
	case strings.Contains(name, "db") || strings.Contains(name, "database") || strings.Contains(name, "cache") || strings.Contains(name, "redis") || strings.Contains(name, "postgres") || strings.Contains(name, "mysql") || strings.Contains(name, "mongo") || strings.Contains(name, "storage"):
		return 0.75
	case strings.Contains(name, "infra") || strings.Contains(name, "platform") || strings.Contains(name, "core") || strings.Contains(name, "k8s") || strings.Contains(name, "kubernetes") || strings.Contains(name, "system"):
		return 1.0
	default:
		return 0.25
	}
}

// hashServiceName generates a deterministic float from a service name using FNV.
func hashServiceName(name string) float64 {
	if name == "" {
		return 0.0
	}
	h := fnv.New64a()
	h.Write([]byte(name))
	hashVal := h.Sum64()
	return float64(hashVal) / float64(uint64(0xFFFFFFFFFFFFFFFF))
}

// inferCategories derives issue categories from title, description, and labels.
func inferCategories(title string, labels map[string]string, hints []string) CategoryFeatures {
	cat := CategoryFeatures{}

	text := strings.ToLower(title)

	hintSet := make(map[string]bool)
	for _, h := range hints {
		hintSet[strings.ToLower(h)] = true
	}

	labelVals := make(map[string]bool)
	if labels != nil {
		for _, v := range labels {
			labelVals[strings.ToLower(v)] = true
		}
		for k, v := range labels {
			labelVals[strings.ToLower(k+":"+v)] = true
		}
	}

	isNetwork := hintSet["network"] || strings.Contains(text, "network") || strings.Contains(text, "dns") || strings.Contains(text, "timeout") || strings.Contains(text, "connection") || strings.Contains(text, "latency") || labelVals["category:network"]
	if isNetwork {
		cat.IsNetworkIssue = 1.0
	}

	isDatabase := hintSet["database"] || hintSet["db"] || strings.Contains(text, "database") || strings.Contains(text, "db-") || strings.Contains(text, "sql") || strings.Contains(text, "query") || strings.Contains(text, "replication") || labelVals["category:database"]
	if isDatabase {
		cat.IsDatabaseIssue = 1.0
	}

	isInfra := hintSet["infra"] || hintSet["infrastructure"] || strings.Contains(text, "infra") || strings.Contains(text, "server") || strings.Contains(text, "node") || strings.Contains(text, "host") || strings.Contains(text, "kubernetes") || strings.Contains(text, "k8s") || labelVals["category:infra"]
	if isInfra {
		cat.IsInfraIssue = 1.0
	}

	isApp := hintSet["app"] || hintSet["application"] || strings.Contains(text, "error") || strings.Contains(text, "exception") || strings.Contains(text, "crash") || strings.Contains(text, "panic") || strings.Contains(text, "oom") || strings.Contains(text, "memory") || labelVals["category:app"]
	if isApp {
		cat.IsAppIssue = 1.0
	}

	isSecurity := hintSet["security"] || strings.Contains(text, "security") || strings.Contains(text, "breach") || strings.Contains(text, "auth") || strings.Contains(text, "vulnerability") || strings.Contains(text, "certificate") || labelVals["category:security"]
	if isSecurity {
		cat.IsSecurityIssue = 1.0
	}

	isConfig := hintSet["config"] || hintSet["configuration"] || strings.Contains(text, "config") || strings.Contains(text, "misconfig") || strings.Contains(text, "setting") || labelVals["category:config"]
	if isConfig {
		cat.IsConfigIssue = 1.0
	}

	isDeployment := hintSet["deployment"] || hintSet["deploy"] || strings.Contains(text, "deploy") || strings.Contains(text, "release") || strings.Contains(text, "rollback") || strings.Contains(text, "canary") || strings.Contains(text, "blue-green") || labelVals["category:deployment"]
	if isDeployment {
		cat.IsDeploymentIssue = 1.0
	}

	isCapacity := hintSet["capacity"] || strings.Contains(text, "capacity") || strings.Contains(text, "scaling") || strings.Contains(text, "throttl") || strings.Contains(text, "rate limit") || strings.Contains(text, "overload") || labelVals["category:capacity"]
	if isCapacity {
		cat.IsCapacityIssue = 1.0
	}

	return cat
}
