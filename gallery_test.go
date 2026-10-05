package solidlens_test

import (
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

func TestGalleryImages(t *testing.T) {
	for _, name := range []string{"hero.png", "mechanical.png", "forms.png"} {
		file, err := os.Open(filepath.Join("docs", "images", name))
		require.NoError(t, err)
		config, format, err := image.DecodeConfig(file)
		closeErr := file.Close()
		require.NoError(t, err)
		require.NoError(t, closeErr)
		require.Equal(t, "png", format)
		require.Equal(t, 1440, config.Width)
		require.Equal(t, 810, config.Height)
	}
}

func TestGalleryModels(t *testing.T) {
	for _, name := range []string{
		"hero-stone.stl",
		"hero-stone-light.stl",
		"hero-soil.stl",
		"hero-soil-light.stl",
		"hero-grass.stl",
		"hero-grass-light.stl",
		"hero-grass-dark.stl",
		"hero-path.stl",
		"hero-path-light.stl",
		"hero-water.stl",
		"hero-wood.stl",
		"hero-dark-wood.stl",
		"hero-leaves.stl",
		"hero-roof.stl",
		"hero-lights.stl",
		"hero-clouds.stl",
		"mechanical-blue.stl",
		"mechanical-gold.3mf",
		"forms-pink.stl",
		"forms-green.stl",
		"forms-blue.stl",
	} {
		file, err := os.Open(filepath.Join("docs", "models", name))
		require.NoError(t, err)
		mesh, err := solidlens.ReadMesh(name, file)
		closeErr := file.Close()
		require.NoError(t, err)
		require.NoError(t, closeErr)
		require.NotEmpty(t, mesh.Triangles())
	}
}
