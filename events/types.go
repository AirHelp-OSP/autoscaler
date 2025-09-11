package events

import (
	"encoding/json"
	"fmt"
)

type ScalingEventData struct {
	CurrentReplicas  int     `json:"current_replicas"`  
	TargetReplicas   int     `json:"target_replicas"`   
	ProbeValue       int     `json:"probe_value"`       
	Threshold        int     `json:"threshold"`         
	LoadPercentage   float64 `json:"load_percentage"`  
	MinPods          int     `json:"min_pods"`          
	MaxPods          int     `json:"max_pods"`         

	ScalingDirection string `json:"scaling_direction"`
	ProbeType        string `json:"probe_type"`       
	ScalingReason    string `json:"scaling_reason"`   

	DeploymentName string `json:"deployment_name"`
	Namespace      string `json:"namespace"`
	Environment    string `json:"environment"`
	Timestamp      int64  `json:"timestamp"`

	HumanMessage string `json:"human_message"`
}

func (e *ScalingEventData) ToJSON() string {
	data, _ := json.Marshal(e)
	return string(data)
}

func (e *ScalingEventData) BuildHumanMessage() string {
	if e.ScalingDirection == "none" {
		return fmt.Sprintf(
			"Remaining at %d replicas | %s: %d/%d (%.1f%%) | Reason: %s",
			e.CurrentReplicas,
			e.ProbeType,
			e.ProbeValue,
			e.Threshold,
			e.LoadPercentage,
			e.ScalingReason,
		)
	}

	return fmt.Sprintf(
		"Scaled %s from %d to %d replicas | %s: %d/%d (%.1f%%) | Reason: %s",
		e.ScalingDirection,
		e.CurrentReplicas,
		e.TargetReplicas,
		e.ProbeType,
		e.ProbeValue,
		e.Threshold,
		e.LoadPercentage,
		e.ScalingReason,
	)
}
