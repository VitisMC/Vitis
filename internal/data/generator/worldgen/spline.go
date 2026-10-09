package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// generatedSpline retains ordered float32 interpolation inputs from the resource graph.
type generatedSpline struct {
	constant   json.RawMessage
	coordinate int
	points     []generatedSplinePoint
}

// generatedSplinePoint identifies one ordered nested value and its exact coefficients.
type generatedSplinePoint struct {
	Location   json.RawMessage `json:"location"`
	Value      json.RawMessage `json:"value"`
	Derivative json.RawMessage `json:"derivative"`
	index      int
}

// splineNode resolves shared spline values and their density coordinates during graph discovery.
func (e *densityEmitter) splineNode(raw json.RawMessage) int {
	key := canonical(raw)
	if index, ok := e.splineIDs[key]; ok {
		return index
	}
	index := len(e.splines)
	e.splineIDs[key] = index
	e.splines = append(e.splines, generatedSpline{})
	if key[0] != '{' {
		e.splines[index].constant = raw
		return index
	}
	var value struct {
		Coordinate json.RawMessage        `json:"coordinate"`
		Points     []generatedSplinePoint `json:"points"`
	}
	must(json.Unmarshal(raw, &value))
	coordinate := e.node(value.Coordinate)
	for n := range value.Points {
		value.Points[n].index = e.splineNode(value.Points[n].Value)
	}
	e.splines[index] = generatedSpline{coordinate: coordinate, points: value.Points}
	return index
}

// floatNumber emits an explicit float32 conversion before runtime spline arithmetic.
func floatNumber(raw json.RawMessage) string { return "float32(" + number(raw) + ")" }

// emitSplines emits direct branches and preserves nested evaluation and float32 rounding order.
func (e *densityEmitter) emitSplines(out *bytes.Buffer) {
	for index, spline := range e.splines {
		fmt.Fprintf(out, "// s%d evaluates one immutable spline with float32 arithmetic.\nfunc(g *standardDensity)s%d(p Position)float32{\n", index, index)
		if spline.constant != nil {
			fmt.Fprintf(out, "return %s\n}\n", floatNumber(spline.constant))
			continue
		}
		fmt.Fprintf(out, "coordinate:=float32(g.n%d(p))\n", spline.coordinate)
		first := spline.points[0]
		fmt.Fprintf(out, "if coordinate < %s {\n", floatNumber(first.Location))
		emitSplineExtend(out, first)
		out.WriteString("}\n")
		for n := 0; n < len(spline.points)-1; n++ {
			a, b := spline.points[n], spline.points[n+1]
			fmt.Fprintf(out, "if coordinate < %s {\n", floatNumber(b.Location))
			fmt.Fprintf(out, "firstLocation,secondLocation := %s,%s\n", floatNumber(a.Location), floatNumber(b.Location))
			fmt.Fprintf(out, "firstDerivative,secondDerivative := %s,%s\n", floatNumber(a.Derivative), floatNumber(b.Derivative))
			out.WriteString("delta:=(coordinate-firstLocation)/(secondLocation-firstLocation)\n")
			fmt.Fprintf(out, "firstValue,secondValue:=g.s%d(p),g.s%d(p)\n", a.index, b.index)
			out.WriteString("firstSlope:=firstDerivative*(secondLocation-firstLocation)-(secondValue-firstValue)\nsecondSlope:=-secondDerivative*(secondLocation-firstLocation)+(secondValue-firstValue)\nlinear:=firstValue+delta*(secondValue-firstValue)\ncorrection:=delta*(1-delta)*(firstSlope+delta*(secondSlope-firstSlope))\nreturn linear+correction\n}\n")
		}
		emitSplineExtend(out, spline.points[len(spline.points)-1])
		out.WriteString("}\n")
	}
}

// emitSplineExtend emits the endpoint's original zero-derivative shortcut.
func emitSplineExtend(out *bytes.Buffer, point generatedSplinePoint) {
	var derivative float32
	must(json.Unmarshal(point.Derivative, &derivative))
	if derivative == 0 {
		fmt.Fprintf(out, "return g.s%d(p)\n", point.index)
		return
	}
	fmt.Fprintf(out, "value:=g.s%d(p)\nreturn value+%s*(coordinate-%s)\n", point.index, floatNumber(point.Derivative), floatNumber(point.Location))
}
