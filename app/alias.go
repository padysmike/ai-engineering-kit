package main

import "math/rand/v2"

var (
	aliasColors  = []string{"Amber", "Blue", "Coral", "Green", "Indigo", "Olive", "Red", "Silver", "Teal", "Violet"}
	aliasAnimals = []string{"Badger", "Crane", "Dolphin", "Fox", "Heron", "Lynx", "Otter", "Owl", "Panda", "Wolf"}
)

func randomAlias() string {
	return aliasColors[rand.IntN(len(aliasColors))] + " " + aliasAnimals[rand.IntN(len(aliasAnimals))]
}
