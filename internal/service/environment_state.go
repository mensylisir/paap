package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"paap/internal/model"

	"gorm.io/gorm"
)

type EnvironmentState struct {
	Environment model.Environment
	Components  []model.Component
	Services    []model.ServiceInstallation
	Infra       []model.InfraInstallation
}

type CanvasNodePosition struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

type CanvasManualEdge struct {
	FromKey string `json:"fromKey"`
	ToKey   string `json:"toKey"`
}

type EnvironmentCanvasStateInput struct {
	Positions map[string]CanvasNodePosition `json:"positions"`
	Edges     []CanvasManualEdge            `json:"edges"`
	Names     map[string]string             `json:"names"`
}

type EnvironmentCanvasStateView struct {
	Positions map[string]CanvasNodePosition `json:"positions"`
	Edges     []CanvasManualEdge            `json:"edges"`
	Names     map[string]string             `json:"names"`
}

func GetEnvironmentState(db *gorm.DB, envID uint) (EnvironmentState, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return EnvironmentState{}, err
	}

	state := EnvironmentState{Environment: env}
	if err := db.Where("environment_id = ?", env.ID).Find(&state.Components).Error; err != nil {
		return EnvironmentState{}, err
	}
	if err := db.Where("environment_id = ?", env.ID).Find(&state.Services).Error; err != nil {
		return EnvironmentState{}, err
	}
	if err := db.Where("environment_id = ?", env.ID).Find(&state.Infra).Error; err != nil {
		return EnvironmentState{}, err
	}
	return state, nil
}

func ListApplicationEnvironments(db *gorm.DB, appID uint) ([]model.Environment, error) {
	app, err := findApplication(db, appID)
	if err != nil {
		return nil, err
	}
	var envs []model.Environment
	if err := db.Where("application_id = ?", app.ID).Find(&envs).Error; err != nil {
		return nil, err
	}
	return envs, nil
}

func ListEnvironmentComponents(db *gorm.DB, envID uint) (model.Environment, []model.Component, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return model.Environment{}, nil, err
	}
	var components []model.Component
	if err := db.Where("environment_id = ?", env.ID).Find(&components).Error; err != nil {
		return model.Environment{}, nil, err
	}
	return env, components, nil
}

func GetEnvironmentCanvasState(db *gorm.DB, envID uint) (EnvironmentCanvasStateView, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return EnvironmentCanvasStateView{}, err
	}

	var state model.EnvironmentCanvasState
	if err := db.Where("environment_id = ?", env.ID).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return EnvironmentCanvasStateView{
				Positions: map[string]CanvasNodePosition{},
				Edges:     []CanvasManualEdge{},
				Names:     map[string]string{},
			}, nil
		}
		return EnvironmentCanvasStateView{}, err
	}

	positions, edges, names := cleanCanvasStateForEnvironment(db, env.ID, valueOrDefaultString(state.Positions, "{}"), valueOrDefaultString(state.Edges, "[]"), valueOrDefaultString(state.Names, "{}"))
	if positions != state.Positions || edges != state.Edges || names != state.Names {
		state.Positions = positions
		state.Edges = edges
		state.Names = names
		if err := db.Save(&state).Error; err != nil {
			return EnvironmentCanvasStateView{}, err
		}
	}
	return decodeCanvasState(state), nil
}

func SaveEnvironmentCanvasState(db *gorm.DB, envID uint, input EnvironmentCanvasStateInput) (EnvironmentCanvasStateView, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return EnvironmentCanvasStateView{}, err
	}

	positionsJSON, err := json.Marshal(normalizeCanvasPositions(input.Positions))
	if err != nil {
		return EnvironmentCanvasStateView{}, ValidationError{Message: "invalid canvas positions"}
	}
	edgesJSON, err := json.Marshal(normalizeCanvasEdges(input.Edges))
	if err != nil {
		return EnvironmentCanvasStateView{}, ValidationError{Message: "invalid canvas edges"}
	}
	namesJSON, err := json.Marshal(normalizeCanvasNames(input.Names))
	if err != nil {
		return EnvironmentCanvasStateView{}, ValidationError{Message: "invalid canvas names"}
	}

	var state model.EnvironmentCanvasState
	err = db.Where("environment_id = ?", env.ID).First(&state).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return EnvironmentCanvasStateView{}, err
	}
	state.EnvironmentID = env.ID
	state.Positions = string(positionsJSON)
	state.Edges = string(edgesJSON)
	state.Names = string(namesJSON)
	if state.ID == 0 {
		err = db.Create(&state).Error
	} else {
		err = db.Save(&state).Error
	}
	if err != nil {
		return EnvironmentCanvasStateView{}, err
	}
	return decodeCanvasState(state), nil
}

func RemoveComponentFromCanvasState(db *gorm.DB, envID uint, componentID uint) error {
	return removeCanvasNodeFromState(db, envID, fmt.Sprintf("component:%d", componentID))
}

func RemoveCapabilityFromCanvasState(db *gorm.DB, envID uint, capabilityID uint) error {
	return removeCanvasNodeFromState(db, envID, fmt.Sprintf("capability:%d", capabilityID))
}

func removeCanvasNodeFromState(db *gorm.DB, envID uint, nodeKey string) error {
	var state model.EnvironmentCanvasState
	if err := db.First(&state, "environment_id = ?", envID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	var positions map[string]json.RawMessage
	if err := json.Unmarshal([]byte(valueOrDefaultString(state.Positions, "{}")), &positions); err == nil {
		deleteCanvasNodeKeys(positions, nodeKey)
		if data, err := json.Marshal(positions); err == nil {
			state.Positions = string(data)
		}
	}

	var edges []map[string]interface{}
	if err := json.Unmarshal([]byte(valueOrDefaultString(state.Edges, "[]")), &edges); err == nil {
		filtered := edges[:0]
		for _, edge := range edges {
			if canvasEdgeTouchesNode(edge, nodeKey) {
				continue
			}
			filtered = append(filtered, edge)
		}
		if data, err := json.Marshal(filtered); err == nil {
			state.Edges = string(data)
		}
	}

	var names map[string]string
	if err := json.Unmarshal([]byte(valueOrDefaultString(state.Names, "{}")), &names); err == nil {
		deleteCanvasNodeKeys(names, nodeKey)
		if data, err := json.Marshal(names); err == nil {
			state.Names = string(data)
		}
	}

	return db.Save(&state).Error
}

func decodeCanvasState(state model.EnvironmentCanvasState) EnvironmentCanvasStateView {
	var positions map[string]CanvasNodePosition
	if err := json.Unmarshal([]byte(valueOrDefaultString(state.Positions, "{}")), &positions); err != nil {
		positions = map[string]CanvasNodePosition{}
	}
	var edges []CanvasManualEdge
	if err := json.Unmarshal([]byte(valueOrDefaultString(state.Edges, "[]")), &edges); err != nil {
		edges = []CanvasManualEdge{}
	}
	var names map[string]string
	if err := json.Unmarshal([]byte(valueOrDefaultString(state.Names, "{}")), &names); err != nil {
		names = map[string]string{}
	}
	return EnvironmentCanvasStateView{Positions: positions, Edges: edges, Names: names}
}

func cleanCanvasStateForEnvironment(db *gorm.DB, envID uint, positionsJSON, edgesJSON, namesJSON string) (string, string, string) {
	validKeys := currentCanvasNodeKeys(db, envID)
	var positions map[string]CanvasNodePosition
	if err := json.Unmarshal([]byte(valueOrDefaultString(positionsJSON, "{}")), &positions); err != nil {
		positions = map[string]CanvasNodePosition{}
	}
	cleanPositions := map[string]CanvasNodePosition{}
	for key, pos := range normalizeCanvasPositions(positions) {
		if validKeys[key] {
			cleanPositions[key] = pos
		}
	}

	var edges []CanvasManualEdge
	if err := json.Unmarshal([]byte(valueOrDefaultString(edgesJSON, "[]")), &edges); err != nil {
		edges = nil
	}
	cleanEdges := make([]CanvasManualEdge, 0, len(edges))
	for _, edge := range normalizeCanvasEdges(edges) {
		if validKeys[edge.FromKey] && validKeys[edge.ToKey] {
			cleanEdges = append(cleanEdges, edge)
		}
	}

	var names map[string]string
	if err := json.Unmarshal([]byte(valueOrDefaultString(namesJSON, "{}")), &names); err != nil {
		names = map[string]string{}
	}
	cleanNames := map[string]string{}
	for key, value := range normalizeCanvasNames(names) {
		if validKeys[key] {
			cleanNames[key] = value
		}
	}

	positionsBytes, err := json.Marshal(cleanPositions)
	if err != nil {
		positionsBytes = []byte("{}")
	}
	edgesBytes, err := json.Marshal(cleanEdges)
	if err != nil {
		edgesBytes = []byte("[]")
	}
	namesBytes, err := json.Marshal(cleanNames)
	if err != nil {
		namesBytes = []byte("{}")
	}
	return string(positionsBytes), string(edgesBytes), string(namesBytes)
}

func deleteCanvasNodeKeys[T any](items map[string]T, nodeKey string) {
	delete(items, nodeKey)
	delete(items, "component:"+nodeKey)
	delete(items, "environment:"+nodeKey)
}

func canvasEdgeTouchesNode(edge map[string]interface{}, nodeKey string) bool {
	return canvasEdgeEndpointMatches(edge["fromKey"], nodeKey) || canvasEdgeEndpointMatches(edge["toKey"], nodeKey)
}

func canvasEdgeEndpointMatches(value interface{}, nodeKey string) bool {
	key := strings.TrimSpace(fmt.Sprint(value))
	return key == nodeKey || key == "component:"+nodeKey || key == "environment:"+nodeKey
}

func currentCanvasNodeKeys(db *gorm.DB, envID uint) map[string]bool {
	keys := map[string]bool{
		"zone:environment":             true,
		"zone:shared":                  true,
		"zone:external":                true,
		"environment:zone:environment": true,
		"environment:zone:shared":      true,
		"environment:zone:external":    true,
	}
	var components []model.Component
	if err := db.Where("environment_id = ?", envID).Find(&components).Error; err == nil {
		for _, comp := range components {
			key := fmt.Sprintf("component:%d", comp.ID)
			keys[key] = true
			keys["component:"+key] = true
			keys["environment:"+key] = true
		}
	}
	var services []model.ServiceInstallation
	if err := db.Where("environment_id = ?", envID).Find(&services).Error; err == nil {
		for _, svc := range services {
			key := fmt.Sprintf("service:%d", svc.ID)
			keys[key] = true
			keys["component:"+key] = true
			keys["environment:"+key] = true
		}
	}
	var infra []model.InfraInstallation
	if err := db.Where("environment_id = ?", envID).Find(&infra).Error; err == nil {
		for _, item := range infra {
			key := fmt.Sprintf("infra:%d", item.ID)
			keys[key] = true
			keys["component:"+key] = true
			keys["environment:"+key] = true
		}
	}
	var capabilities []model.EnvironmentCapability
	if err := db.Where("environment_id = ?", envID).Find(&capabilities).Error; err == nil {
		for _, capability := range capabilities {
			key := fmt.Sprintf("capability:%d", capability.ID)
			keys[key] = true
			keys["component:"+key] = true
			keys["environment:"+key] = true
		}
	}
	return keys
}

func normalizeCanvasPositions(input map[string]CanvasNodePosition) map[string]CanvasNodePosition {
	out := map[string]CanvasNodePosition{}
	for key, pos := range input {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		normalized := CanvasNodePosition{X: pos.X, Y: pos.Y}
		if pos.Width > 0 {
			normalized.Width = pos.Width
		}
		if pos.Height > 0 {
			normalized.Height = pos.Height
		}
		out[key] = normalized
	}
	return out
}

func normalizeCanvasEdges(input []CanvasManualEdge) []CanvasManualEdge {
	seen := map[string]struct{}{}
	out := make([]CanvasManualEdge, 0, len(input))
	for _, edge := range input {
		from := strings.TrimSpace(edge.FromKey)
		to := strings.TrimSpace(edge.ToKey)
		if from == "" || to == "" || from == to {
			continue
		}
		key := from + "\x00" + to
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, CanvasManualEdge{FromKey: from, ToKey: to})
	}
	return out
}

func normalizeCanvasNames(input map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}
