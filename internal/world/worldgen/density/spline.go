package density

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

type spline interface {
	sample(Position) float32
}

type constantSpline float32

func (s constantSpline) sample(Position) float32 { return float32(s) }

type splinePoint struct {
	location   float32
	value      spline
	derivative float32
}

type multipointSpline struct {
	coordinate Function
	points     []splinePoint
}

func (s *multipointSpline) sample(position Position) float32 {
	coordinate := float32(s.coordinate.Sample(position))
	index := sort.Search(len(s.points), func(index int) bool { return coordinate < s.points[index].location }) - 1
	if index < 0 {
		return splineLinearExtend(coordinate, s.points[0], position)
	}
	if index == len(s.points)-1 {
		return splineLinearExtend(coordinate, s.points[index], position)
	}
	first, second := s.points[index], s.points[index+1]
	delta := (coordinate - first.location) / (second.location - first.location)
	firstValue, secondValue := first.value.sample(position), second.value.sample(position)
	firstSlope := first.derivative*(second.location-first.location) - (secondValue - firstValue)
	secondSlope := -second.derivative*(second.location-first.location) + (secondValue - firstValue)
	linear := firstValue + delta*(secondValue-firstValue)
	correction := delta * (1 - delta) * (firstSlope + delta*(secondSlope-firstSlope))
	return linear + correction
}

func splineLinearExtend(coordinate float32, point splinePoint, position Position) float32 {
	value := point.value.sample(position)
	if point.derivative == 0 {
		return value
	}
	return value + point.derivative*(coordinate-point.location)
}

func (c *Compiler) compileSplineFunction(data json.RawMessage) (Function, error) {
	var value struct {
		Spline json.RawMessage `json:"spline"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	sampler, err := c.compileSpline(value.Spline)
	if err != nil {
		return nil, err
	}
	return functionFunc(func(position Position) float64 { return float64(sampler.sample(position)) }), nil
}

func (c *Compiler) compileSpline(data json.RawMessage) (spline, error) {
	data = trimJSON(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("empty spline")
	}
	if data[0] != '{' {
		var value float32
		if err := json.Unmarshal(data, &value); err != nil || math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, fmt.Errorf("invalid spline constant")
		}
		return constantSpline(value), nil
	}
	var value struct {
		Coordinate json.RawMessage `json:"coordinate"`
		Points     []struct {
			Location   float32         `json:"location"`
			Value      json.RawMessage `json:"value"`
			Derivative float32         `json:"derivative"`
		} `json:"points"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	if len(value.Points) == 0 {
		return nil, fmt.Errorf("spline has no points")
	}
	coordinate, err := c.compile(value.Coordinate)
	if err != nil {
		return nil, fmt.Errorf("spline coordinate: %w", err)
	}
	result := &multipointSpline{coordinate: coordinate, points: make([]splinePoint, len(value.Points))}
	for index, point := range value.Points {
		if math.IsNaN(float64(point.Location)) || math.IsInf(float64(point.Location), 0) || math.IsNaN(float64(point.Derivative)) || math.IsInf(float64(point.Derivative), 0) {
			return nil, fmt.Errorf("spline point %d is non-finite", index)
		}
		if index > 0 && point.Location <= value.Points[index-1].Location {
			return nil, fmt.Errorf("spline locations are not strictly increasing")
		}
		pointValue, err := c.compileSpline(point.Value)
		if err != nil {
			return nil, fmt.Errorf("spline point %d: %w", index, err)
		}
		result.points[index] = splinePoint{location: point.Location, value: pointValue, derivative: point.Derivative}
	}
	return result, nil
}
