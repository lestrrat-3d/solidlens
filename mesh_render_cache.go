package solidlens

import "image/color"

type meshRenderKey struct {
	camera   Camera
	settings Settings
	front    Material
	back     Material
	lights   lighting
}

func (key meshRenderKey) equal(other meshRenderKey) bool {
	if key.camera != other.camera || key.settings != other.settings ||
		key.front != other.front || key.back != other.back ||
		len(key.lights.directional) != len(other.lights.directional) ||
		len(key.lights.points) != len(other.lights.points) {
		return false
	}
	for index, light := range key.lights.directional {
		if light != other.lights.directional[index] {
			return false
		}
	}
	for index, light := range key.lights.points {
		if light != other.lights.points[index] {
			return false
		}
	}
	return true
}

type cachedTriangle struct {
	projected [3]screenVertex
	pixel     color.RGBA
	state     rasterState
}

type meshRenderCache struct {
	key       meshRenderKey
	vertices  []screenVertex
	triangles []cachedTriangle
}

type edgeCoverageKey struct {
	// Coverage depends on geometry, view, width, and crease angle. Color and
	// material are applied after coverage is calculated.
	camera      Camera
	settings    Settings
	width       float64
	creaseAngle float64
}

type edgeCoverageCache struct {
	key    edgeCoverageKey
	canvas *edgeCanvas
}

func (m *Mesh) prepareRender(v view, camera Camera, settings Settings, front, back Material, lights lighting) *meshRenderCache {
	key := meshRenderKey{camera: camera, settings: settings, front: front, back: back, lights: lights}
	if m.cache != nil {
		if cached := m.cache.render.Load(); cached != nil && cached.key.equal(key) {
			return cached
		}
	}
	prepared := &meshRenderCache{key: key, triangles: make([]cachedTriangle, len(m.triangles))}
	var projectedReady []bool
	if len(m.triangles) >= len(m.vertices) && len(m.vertices) > 0 {
		prepared.vertices = make([]screenVertex, len(m.vertices))
		projectedReady = make([]bool, len(m.vertices))
	}
	for triangleIndex, triangle := range m.triangles {
		world := [3]Vec{m.vertices[triangle[0]], m.vertices[triangle[1]], m.vertices[triangle[2]]}
		normal, ok := world[1].Sub(world[0]).Cross(world[2].Sub(world[0])).Normalize()
		if !ok {
			continue
		}
		material := front
		if normal.Dot(camera.Position.Sub(world[0])) < 0 {
			normal = normal.Scale(-1)
			material = back
		}
		shade := shadeTriangle(lights, material, normal, world[0])
		var projected [3]screenVertex
		var visible bool
		if prepared.vertices == nil {
			projected, visible = v.projectTriangle(world)
		} else {
			visible = true
			for corner, index := range triangle {
				if !projectedReady[index] {
					prepared.vertices[index] = v.projectClippedPoint(m.vertices[index])
					projectedReady[index] = true
				}
				projected[corner] = prepared.vertices[index]
				if projected[corner].z < v.near || projected[corner].z > v.far {
					visible = false
				}
			}
		}
		if visible {
			state := newRasterState(settings.Width, settings.Height, projected)
			if state.valid {
				prepared.triangles[triangleIndex] = cachedTriangle{
					projected: projected,
					pixel:     premultiply(shade.NRGBA()),
					state:     state,
				}
			}
		}
	}
	if m.cache != nil {
		m.cache.render.Store(prepared)
	}
	return prepared
}
