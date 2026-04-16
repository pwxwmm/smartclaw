package topology

import (
	"fmt"
	"time"
)

// SpanData represents an OTLP span extracted for topology inference.
type SpanData struct {
	ServiceName string            `json:"service_name"`
	PeerService string            `json:"peer_service,omitempty"`
	PeerAddress string            `json:"peer_address,omitempty"`
	SpanKind    string            `json:"span_kind"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

// SOPAInventoryItem represents an item from SOPA inventory.
type SOPAInventoryItem struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Labels   map[string]string `json:"labels,omitempty"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

// K8sResource represents a Kubernetes resource for topology discovery.
type K8sResource struct {
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
	Selector  map[string]string `json:"selector,omitempty"`
}

// IngestOTLPSpans extracts topology edges from OTLP span data.
// It processes CLIENT spans to infer service-to-service dependencies:
//   - Source = service.name (caller)
//   - Target = server.address or service.peer.name (callee)
//   - Edge type = calls or depends_on based on span kind
func IngestOTLPSpans(graph *TopologyGraph, spans []SpanData) {
	if graph == nil {
		return
	}

	now := time.Now()
	for _, span := range spans {
		if span.ServiceName == "" {
			continue
		}
		if span.SpanKind != "client" && span.SpanKind != "CLIENT" && span.SpanKind != "Client" {
			continue
		}

		target := span.PeerService
		if target == "" {
			target = span.PeerAddress
		}
		if target == "" {
			if addr, ok := span.Attributes["server.address"]; ok && addr != "" {
				target = addr
			}
		}
		if target == "" {
			if peer, ok := span.Attributes["service.peer.name"]; ok && peer != "" {
				target = peer
			}
		}
		if target == "" {
			continue
		}

		sourceID := "svc:" + span.ServiceName
		targetID := "svc:" + target

		graph.AddNode(&Node{
			ID:       sourceID,
			Type:     NodeTypeService,
			Name:     span.ServiceName,
			Health:   HealthUnknown,
			LastSeen: now,
			Labels:   map[string]string{"source": "otlp"},
		})

		graph.AddNode(&Node{
			ID:       targetID,
			Type:     NodeTypeService,
			Name:     target,
			Health:   HealthUnknown,
			LastSeen: now,
			Labels:   map[string]string{"source": "otlp"},
		})

		edgeType := EdgeCalls
		if span.SpanKind == "depends_on" {
			edgeType = EdgeDependsOn
		}

		graph.AddEdge(&Edge{
			Source:   sourceID,
			Target:   targetID,
			Type:     edgeType,
			Weight:   0.5,
			LastSeen: now,
			Labels:   map[string]string{"protocol": span.Attributes["rpc.system"], "source": "otlp"},
		})
	}
}

// IngestSOPAInventory converts SOPA inventory items into topology nodes.
func IngestSOPAInventory(graph *TopologyGraph, items []SOPAInventoryItem) {
	if graph == nil {
		return
	}

	now := time.Now()
	for _, item := range items {
		if item.ID == "" {
			continue
		}

		nodeType := nodeTypeFromString(item.Type)

		labels := map[string]string{"source": "sopa"}
		for k, v := range item.Labels {
			labels[k] = v
		}

		metadata := map[string]any{}
		for k, v := range item.Metadata {
			metadata[k] = v
		}

		graph.AddNode(&Node{
			ID:       item.ID,
			Type:     nodeType,
			Name:     item.Name,
			Health:   HealthUnknown,
			LastSeen: now,
			Labels:   labels,
			Metadata: metadata,
		})
	}
}

// IngestK8sResources processes Kubernetes resources and derives topology relationships:
//   - deployment → pods (via label selector)
//   - service → pods (via label selector)
//   - pods → host (via node name)
func IngestK8sResources(graph *TopologyGraph, resources []K8sResource) {
	if graph == nil {
		return
	}

	now := time.Now()

	var deployments []K8sResource
	var services []K8sResource
	var pods []K8sResource
	var namespaces []K8sResource

	for _, res := range resources {
		switch res.Kind {
		case "Deployment":
			deployments = append(deployments, res)
		case "Service":
			services = append(services, res)
		case "Pod":
			pods = append(pods, res)
		case "Namespace":
			namespaces = append(namespaces, res)
		}
	}

	for _, ns := range namespaces {
		nsID := fmt.Sprintf("ns:%s", ns.Name)
		graph.AddNode(&Node{
			ID:       nsID,
			Type:     NodeTypeK8sNamespace,
			Name:     ns.Name,
			Health:   HealthUnknown,
			LastSeen: now,
			Labels:   mergeLabels(ns.Labels, map[string]string{"source": "k8s", "namespace": ns.Name}),
		})
	}

	for _, dep := range deployments {
		depID := fmt.Sprintf("deploy:%s/%s", dep.Namespace, dep.Name)
		graph.AddNode(&Node{
			ID:       depID,
			Type:     NodeTypeK8sDeployment,
			Name:     dep.Name,
			Health:   HealthUnknown,
			LastSeen: now,
			Labels:   mergeLabels(dep.Labels, map[string]string{"source": "k8s", "namespace": dep.Namespace}),
		})

		nsID := fmt.Sprintf("ns:%s", dep.Namespace)
		graph.AddEdge(&Edge{
			Source:   depID,
			Target:   nsID,
			Type:     EdgeDeployedOn,
			Weight:   1.0,
			LastSeen: now,
			Labels:   map[string]string{"source": "k8s"},
		})

		for _, pod := range pods {
			if pod.Namespace != dep.Namespace {
				continue
			}
			if selectorMatches(dep.Selector, pod.Labels) {
				podID := fmt.Sprintf("pod:%s/%s", pod.Namespace, pod.Name)
				graph.AddNode(&Node{
					ID:       podID,
					Type:     NodeTypeK8sPod,
					Name:     pod.Name,
					Health:   HealthUnknown,
					LastSeen: now,
					Labels:   mergeLabels(pod.Labels, map[string]string{"source": "k8s", "namespace": pod.Namespace}),
				})
				graph.AddEdge(&Edge{
					Source:   depID,
					Target:   podID,
					Type:     EdgeDeployedOn,
					Weight:   1.0,
					LastSeen: now,
					Labels:   map[string]string{"source": "k8s"},
				})
			}
		}
	}

	for _, svc := range services {
		svcID := fmt.Sprintf("svc:%s/%s", svc.Namespace, svc.Name)
		graph.AddNode(&Node{
			ID:       svcID,
			Type:     NodeTypeService,
			Name:     svc.Name,
			Health:   HealthUnknown,
			LastSeen: now,
			Labels:   mergeLabels(svc.Labels, map[string]string{"source": "k8s", "namespace": svc.Namespace}),
		})

		for _, pod := range pods {
			if pod.Namespace != svc.Namespace {
				continue
			}
			if selectorMatches(svc.Selector, pod.Labels) {
				podID := fmt.Sprintf("pod:%s/%s", pod.Namespace, pod.Name)
				graph.AddNode(&Node{
					ID:       podID,
					Type:     NodeTypeK8sPod,
					Name:     pod.Name,
					Health:   HealthUnknown,
					LastSeen: now,
					Labels:   mergeLabels(pod.Labels, map[string]string{"source": "k8s", "namespace": pod.Namespace}),
				})
				graph.AddEdge(&Edge{
					Source:   svcID,
					Target:   podID,
					Type:     EdgeRoutesTo,
					Weight:   0.8,
					LastSeen: now,
					Labels:   map[string]string{"source": "k8s"},
				})
			}
		}
	}

	for _, pod := range pods {
		podID := fmt.Sprintf("pod:%s/%s", pod.Namespace, pod.Name)
		graph.AddNode(&Node{
			ID:       podID,
			Type:     NodeTypeK8sPod,
			Name:     pod.Name,
			Health:   HealthUnknown,
			LastSeen: now,
			Labels:   mergeLabels(pod.Labels, map[string]string{"source": "k8s", "namespace": pod.Namespace}),
		})

		if nodeName, ok := pod.Labels["kubernetes.io/hostname"]; ok && nodeName != "" {
			hostID := "host:" + nodeName
			graph.AddNode(&Node{
				ID:       hostID,
				Type:     NodeTypeHost,
				Name:     nodeName,
				Health:   HealthUnknown,
				LastSeen: now,
				Labels:   map[string]string{"source": "k8s"},
			})
			graph.AddEdge(&Edge{
				Source:   podID,
				Target:   hostID,
				Type:     EdgeDeployedOn,
				Weight:   1.0,
				LastSeen: now,
				Labels:   map[string]string{"source": "k8s"},
			})
		}
	}
}

func nodeTypeFromString(s string) NodeType {
	switch s {
	case "service", "Service":
		return NodeTypeService
	case "database", "Database", "db":
		return NodeTypeDatabase
	case "queue", "Queue":
		return NodeTypeQueue
	case "cache", "Cache":
		return NodeTypeCache
	case "load_balancer", "LoadBalancer", "loadbalancer":
		return NodeTypeLoadBalancer
	case "k8s_cluster", "Cluster":
		return NodeTypeK8sCluster
	case "k8s_namespace", "Namespace":
		return NodeTypeK8sNamespace
	case "k8s_deployment", "Deployment":
		return NodeTypeK8sDeployment
	case "k8s_pod", "Pod":
		return NodeTypeK8sPod
	case "host", "Host", "Node":
		return NodeTypeHost
	default:
		return NodeTypeService
	}
}

func selectorMatches(selector, labels map[string]string) bool {
	for k, v := range selector {
		if lv, ok := labels[k]; !ok || lv != v {
			return false
		}
	}
	return true
}

func mergeLabels(base, overlay map[string]string) map[string]string {
	result := make(map[string]string, len(base)+len(overlay))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v
	}
	return result
}
