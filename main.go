package main

func main() {
	var ecs ECS

	gs := gridSystem{
		level: gridTile{
			pos:   point{-4, -4},
			shape: point{8, 8},
			data: []uint8{
				1, 1, 1, 1, 1, 1, 1, 1,
				1, 0, 0, 0, 0, 0, 0, 1,
				1, 0, 0, 0, 0, 0, 0, 1,
				1, 0, 0, 0, 0, 0, 0, 1,
				1, 0, 0, 0, 0, 0, 0, 1,
				1, 0, 0, 0, 0, 0, 0, 1,
				1, 0, 0, 0, 0, 0, 0, 1,
				1, 1, 1, 1, 1, 1, 1, 1,
			},
		},
	}
	ecs.AddSystem(gs)

	var (
		// world Entity
		player Entity
		// mobs   []Entity
	)

	gs.AddPosition(&player, point{0, 0})

}
