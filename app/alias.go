package main

import (
	"errors"
	"math/rand/v2"
	"slices"
)

var errNoAliasesLeft = errors.New("no aliases left")

var (
	aliasColors = []string{
		"Amber", "Aqua", "Azure", "Beige", "Black", "Blue", "Bronze", "Brown", "Coral", "Crimson",
		"Cyan", "Denim", "Emerald", "Fuchsia", "Gold", "Gray", "Green", "Indigo", "Ivory", "Jade",
		"Khaki", "Lavender", "Lemon", "Lilac", "Lime", "Magenta", "Maroon", "Mint", "Navy", "Ochre",
		"Olive", "Orange", "Peach", "Pink", "Plum", "Purple", "Red", "Silver", "Teal", "Violet",
	}
	aliasAnimals = []string{
		"Badger", "Bear", "Beaver", "Bison", "Camel", "Cheetah", "Cougar", "Crane", "Deer", "Dolphin",
		"Eagle", "Falcon", "Ferret", "Fox", "Gazelle", "Gecko", "Giraffe", "Hedgehog", "Heron", "Koala",
		"Lemur", "Leopard", "Lion", "Lynx", "Moose", "Otter", "Owl", "Panda", "Panther", "Parrot",
		"Penguin", "Rabbit", "Raccoon", "Raven", "Seal", "Tiger", "Toucan", "Turtle", "Wolf", "Zebra",
	}
)

func randomAlias() string {
	return aliasColors[rand.IntN(len(aliasColors))] + " " + aliasAnimals[rand.IntN(len(aliasAnimals))]
}

func drawUnusedAlias(draw func() string, used []string) (string, error) {
	if len(used) >= len(aliasColors)*len(aliasAnimals) {
		return "", errNoAliasesLeft
	}
	alias := draw()
	for slices.Contains(used, alias) {
		alias = draw()
	}
	return alias, nil
}
