package solidlens_test

import (
	"bytes"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	tmf "github.com/lestrrat-go/3mf"
	"github.com/stretchr/testify/require"
)

func TestReadOBJTriangulatesPolygonAndNegativeIndices(t *testing.T) {
	mesh, err := solidlens.ReadOBJ(bytes.NewBufferString("v 0 0 0\nv 1 0 0\nv 1 1 0\nv 0 1 0\nf -4 -3 -2 -1\n"))
	require.NoError(t, err)
	require.Len(t, mesh.Vertices(), 4)
	require.Equal(t, [][3]int{{0, 1, 2}, {0, 2, 3}}, mesh.Triangles())
}

func TestReadSTLUsesStreamingDecoder(t *testing.T) {
	data := "solid triangle\nfacet normal 0 0 1\nouter loop\nvertex 0 0 0\nvertex 1 0 0\nvertex 0 1 0\nendloop\nendfacet\nendsolid triangle\n"
	mesh, err := solidlens.ReadSTL(bytes.NewBufferString(data))
	require.NoError(t, err)
	require.Equal(t, []solidlens.Vec{{}, {X: 1}, {Y: 1}}, mesh.Vertices())
	require.Equal(t, [][3]int{{0, 1, 2}}, mesh.Triangles())
}

func TestRead3MFAppliesBuildTransformAndUnits(t *testing.T) {
	mesh := tmf.NewMesh(
		tmf.WithVertices([]tmf.Vertex{{}, {X: 1}, {Y: 1}}),
		tmf.WithTriangles([]tmf.Triangle{{V1: 0, V2: 1, V3: 2}}),
	)
	object := tmf.NewObject(tmf.WithObjectID(7), tmf.WithMesh(mesh))
	model := tmf.NewModel(
		tmf.WithUnit(tmf.UnitCentimeter),
		tmf.WithObject(object),
		tmf.WithBuildItem(tmf.NewBuildItem(
			tmf.WithObjectRef(object),
			tmf.WithItemTransform(tmf.Matrix{1, 0, 0, 0, 1, 0, 0, 0, 1, 2, 0, 0}),
		)),
	)
	pkg := tmf.NewPackage(tmf.WithModel(model))
	var data bytes.Buffer
	_, err := pkg.WriteTo(&data)
	require.NoError(t, err)

	parsed, err := solidlens.Read3MF(&data)
	require.NoError(t, err)
	require.Equal(t, []solidlens.Vec{{X: 20}, {X: 30}, {X: 20, Y: 10}}, parsed.Vertices())
	require.Equal(t, [][3]int{{0, 1, 2}}, parsed.Triangles())
}
