package solidlens

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	tmf "github.com/lestrrat-go/3mf"
	"github.com/lestrrat-go/stl"
)

// ReadMesh reads an STL, OBJ, or 3MF mesh selected by its filename extension.
func ReadMesh(name string, r io.Reader) (*Mesh, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".stl":
		return ReadSTL(r)
	case ".obj":
		return ReadOBJ(r)
	case ".3mf":
		return Read3MF(r)
	default:
		return nil, fmt.Errorf("solidlens: unsupported mesh format %q", filepath.Ext(name))
	}
}

// ReadSTL reads ASCII or binary STL geometry with github.com/lestrrat-go/stl.
func ReadSTL(r io.Reader) (*Mesh, error) {
	reader := stl.NewReader(r)
	vertices := make([]Vec, 0)
	triangles := make([][3]int, 0)
	for {
		triangle, err := reader.ReadTriangle()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("solidlens: read STL: %w", err)
		}
		index := len(vertices)
		for _, vertex := range triangle.Vertices {
			vertices = append(vertices, Vec{
				X: float64(vertex[0]),
				Y: float64(vertex[1]),
				Z: float64(vertex[2]),
			})
		}
		triangles = append(triangles, [3]int{index, index + 1, index + 2})
	}
	return NewMesh(vertices, triangles)
}

// ReadOBJ reads Wavefront OBJ vertex and face records. Polygon faces are
// triangulated as a fan. Material records are intentionally left to Scene.
func ReadOBJ(r io.Reader) (*Mesh, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 4*1024*1024)
	vertices := make([]Vec, 0)
	triangles := make([][3]int, 0)
	for line := 1; scanner.Scan(); line++ {
		fields := strings.Fields(strings.Split(scanner.Text(), "#")[0])
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "v":
			if len(fields) < 4 || len(fields) > 5 {
				return nil, fmt.Errorf("solidlens: OBJ line %d has an invalid vertex", line)
			}
			vertex, err := parseVec(fields[1:4])
			if err != nil {
				return nil, fmt.Errorf("solidlens: OBJ line %d: %w", line, err)
			}
			if len(fields) == 5 {
				w, err := strconv.ParseFloat(fields[4], 64)
				if err != nil || w == 0 || math.IsNaN(w) || math.IsInf(w, 0) {
					return nil, fmt.Errorf("solidlens: OBJ line %d has an invalid homogeneous coordinate", line)
				}
				vertex = vertex.Scale(1 / w)
			}
			vertices = append(vertices, vertex)
		case "f":
			if len(fields) < 4 {
				return nil, fmt.Errorf("solidlens: OBJ line %d has a face with fewer than three vertices", line)
			}
			indices := make([]int, len(fields)-1)
			for i, field := range fields[1:] {
				index, err := objVertexIndex(field, len(vertices))
				if err != nil {
					return nil, fmt.Errorf("solidlens: OBJ line %d: %w", line, err)
				}
				indices[i] = index
			}
			for i := 1; i+1 < len(indices); i++ {
				triangles = append(triangles, [3]int{indices[0], indices[i], indices[i+1]})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("solidlens: read OBJ: %w", err)
	}
	return NewMesh(vertices, triangles)
}

func objVertexIndex(field string, count int) (int, error) {
	value := strings.Split(field, "/")[0]
	index, err := strconv.Atoi(value)
	if err != nil || index == 0 {
		return 0, fmt.Errorf("invalid vertex index %q", field)
	}
	if index < 0 {
		index = count + index
	} else {
		index--
	}
	if index < 0 || index >= count {
		return 0, fmt.Errorf("vertex index %q is out of range", field)
	}
	return index, nil
}

// Read3MF reads a 3MF package with github.com/lestrrat-go/3mf. It resolves
// build items and nested components, applying their transforms in order.
func Read3MF(r io.Reader) (*Mesh, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("solidlens: read 3MF: %w", err)
	}
	pkg, err := tmf.ReadPackage(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("solidlens: read 3MF package: %w", err)
	}
	return meshFrom3MFModel(pkg.Model())
}

func meshFrom3MFModel(model *tmf.Model) (*Mesh, error) {
	if model == nil {
		return nil, fmt.Errorf("solidlens: 3MF package has no model")
	}
	scale, err := millimetreScale(model.Unit())
	if err != nil {
		return nil, err
	}
	objects := make(map[uint32]*tmf.Object, len(model.Resources().Objects()))
	for _, object := range model.Resources().Objects() {
		objects[object.ID()] = object
	}
	vertices := make([]Vec, 0)
	triangles := make([][3]int, 0)
	var appendObject func(*tmf.Object, tmf.Matrix, map[uint32]struct{}) error
	appendObject = func(object *tmf.Object, transform tmf.Matrix, ancestors map[uint32]struct{}) error {
		if object == nil {
			return fmt.Errorf("solidlens: 3MF references a missing object")
		}
		if _, exists := ancestors[object.ID()]; exists {
			return fmt.Errorf("solidlens: 3MF object %d has a component cycle", object.ID())
		}
		if mesh := object.Mesh(); mesh != nil {
			base := len(vertices)
			for _, vertex := range mesh.Vertices() {
				position := apply3MFTransform(transform, Vec{X: vertex.X, Y: vertex.Y, Z: vertex.Z})
				vertices = append(vertices, position.Scale(scale))
			}
			for _, triangle := range mesh.Triangles() {
				triangles = append(triangles, [3]int{
					base + int(triangle.V1),
					base + int(triangle.V2),
					base + int(triangle.V3),
				})
			}
			return nil
		}
		ancestors[object.ID()] = struct{}{}
		defer delete(ancestors, object.ID())
		for _, component := range object.Components() {
			if component.Path != "" {
				return fmt.Errorf("solidlens: 3MF component %d targets model part %q", component.ObjectID, component.Path)
			}
			child := objects[component.ObjectID]
			if err := appendObject(child, compose3MFTransform(component.Transform, transform), ancestors); err != nil {
				return err
			}
		}
		return nil
	}
	items := model.Build().Items
	if len(items) == 0 {
		for _, object := range model.Resources().Objects() {
			if err := appendObject(object, tmf.IdentityMatrix(), map[uint32]struct{}{}); err != nil {
				return nil, err
			}
		}
	} else {
		for _, item := range items {
			if item.Path != "" {
				return nil, fmt.Errorf("solidlens: 3MF build item %d targets model part %q", item.ObjectID, item.Path)
			}
			if err := appendObject(objects[item.ObjectID], item.Transform, map[uint32]struct{}{}); err != nil {
				return nil, err
			}
		}
	}
	return NewMesh(vertices, triangles)
}

func millimetreScale(unit tmf.Unit) (float64, error) {
	switch unit {
	case tmf.UnitMicron:
		return 0.001, nil
	case tmf.UnitMillimeter:
		return 1, nil
	case tmf.UnitCentimeter:
		return 10, nil
	case tmf.UnitInch:
		return 25.4, nil
	case tmf.UnitFoot:
		return 304.8, nil
	case tmf.UnitMeter:
		return 1000, nil
	default:
		return 0, fmt.Errorf("solidlens: 3MF uses unsupported unit %q", unit)
	}
}

func apply3MFTransform(m tmf.Matrix, v Vec) Vec {
	return Vec{
		X: v.X*m[0] + v.Y*m[3] + v.Z*m[6] + m[9],
		Y: v.X*m[1] + v.Y*m[4] + v.Z*m[7] + m[10],
		Z: v.X*m[2] + v.Y*m[5] + v.Z*m[8] + m[11],
	}
}

// compose3MFTransform returns the transform that first applies a and then b.
func compose3MFTransform(a, b tmf.Matrix) tmf.Matrix {
	var combined tmf.Matrix
	for row := 0; row < 3; row++ {
		for column := 0; column < 3; column++ {
			combined[row*3+column] = a[row*3]*b[column] +
				a[row*3+1]*b[3+column] +
				a[row*3+2]*b[6+column]
		}
	}
	for column := 0; column < 3; column++ {
		combined[9+column] = a[9]*b[column] + a[10]*b[3+column] + a[11]*b[6+column] + b[9+column]
	}
	return combined
}

func parseVec(fields []string) (Vec, error) {
	if len(fields) != 3 {
		return Vec{}, fmt.Errorf("want three coordinates")
	}
	values := [3]float64{}
	for i, field := range fields {
		value, err := strconv.ParseFloat(field, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return Vec{}, fmt.Errorf("coordinate %q is invalid", field)
		}
		values[i] = value
	}
	return Vec{X: values[0], Y: values[1], Z: values[2]}, nil
}
