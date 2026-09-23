// Package solidlens renders triangle meshes into raster images without a
// windowing system or graphics driver.
//
// It reads STL, 3MF, and OBJ files, and it accepts any mesh that exposes the
// same Vertices and Triangles accessors as github.com/lestrrat-3d/decad.Mesh.
// Both sides of every triangle are lit, so open meshes such as decad sheet
// bodies render their inner side too.
package solidlens
