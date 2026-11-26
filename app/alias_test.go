package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func openRoomIn(t *testing.T, rooms *roomStore) string {
	t.Helper()
	code := rooms.create().code
	if _, err := rooms.transition(code, roomDraft, roomOpen); err != nil {
		t.Fatal(err)
	}
	return code
}

func joinTimes(t *testing.T, rooms *roomStore, code string, count int) []string {
	t.Helper()
	aliases := make([]string, 0, count)
	for range count {
		joined, err := rooms.join(code)
		if err != nil {
			t.Fatal(err)
		}
		aliases = append(aliases, joined.alias)
	}
	return aliases
}

func scriptedAliases(aliases ...string) func() string {
	return func() string {
		next := aliases[0]
		aliases = aliases[1:]
		return next
	}
}

func TestAliasWords_BothLists_Hold40DistinctWords(t *testing.T) {
	for name, words := range map[string][]string{"colours": aliasColors, "animals": aliasAnimals} {
		distinct := slices.Compact(slices.Sorted(slices.Values(words)))

		if len(words) != 40 || len(distinct) != 40 {
			t.Errorf("%s: %d words, %d distinct, want 40 of each", name, len(words), len(distinct))
		}
	}
}

func TestJoin_DrawnAliasIsTaken_AnotherAliasIsDrawn(t *testing.T) {
	rooms := newRoomStore(randomRoomCode)
	rooms.newAlias = scriptedAliases("Blue Fox", "Blue Fox", "Blue Fox", "Red Owl")
	code := openRoomIn(t, rooms)

	aliases := joinTimes(t, rooms, code, 2)

	if want := []string{"Blue Fox", "Red Owl"}; !slices.Equal(aliases, want) {
		t.Errorf("aliases = %q, want %q", aliases, want)
	}
}

func TestJoin_AliasIsUsedInAnotherRoom_ItIsIssued(t *testing.T) {
	rooms := newRoomStore(randomRoomCode)
	rooms.newAlias = scriptedAliases("Blue Fox", "Blue Fox")
	first, second := openRoomIn(t, rooms), openRoomIn(t, rooms)
	joinTimes(t, rooms, first, 1)

	aliases := joinTimes(t, rooms, second, 1)

	if aliases[0] != "Blue Fox" {
		t.Errorf("alias = %q, want %q", aliases[0], "Blue Fox")
	}
}

// A kick removes the participant, and their posts keep showing the alias.
func TestJoin_ParticipantIsRemoved_AliasStaysUsed(t *testing.T) {
	rooms := newRoomStore(randomRoomCode)
	rooms.newAlias = scriptedAliases("Blue Fox", "Blue Fox", "Red Owl")
	code := openRoomIn(t, rooms)
	joinTimes(t, rooms, code, 1)
	emptied := rooms.byCode[code]
	emptied.participants = nil
	rooms.byCode[code] = emptied

	aliases := joinTimes(t, rooms, code, 1)

	if aliases[0] != "Red Owl" {
		t.Errorf("alias = %q, want %q", aliases[0], "Red Owl")
	}
}

func TestJoin_EveryAliasIsIssued_NoAliasesLeft(t *testing.T) {
	rooms := newRoomStore(randomRoomCode)
	code := openRoomIn(t, rooms)
	aliases := joinTimes(t, rooms, code, 1600)

	_, err := rooms.join(code)

	if !errors.Is(err, errNoAliasesLeft) {
		t.Errorf("error = %v, want %v", err, errNoAliasesLeft)
	}
	if distinct := slices.Compact(slices.Sorted(slices.Values(aliases))); len(distinct) != 1600 {
		t.Errorf("%d different aliases, want 1600", len(distinct))
	}
	if found, _ := rooms.find(code); len(found.participants) != 1600 {
		t.Errorf("participants = %d, want 1600", len(found.participants))
	}
}

func TestJoinRequest_Refused_ConflictTellsWhy(t *testing.T) {
	tests := []struct {
		name  string
		close bool
		want  string
	}{
		{"WHEN no alias is left THEN the join is a conflict saying so", false, "no aliases left"},
		{"WHEN no alias is left AND the room is closed THEN the conflict says the room is not open", true, "room is not open"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rooms := newRoomStore(randomRoomCode)
			code := openRoomIn(t, rooms)
			joinTimes(t, rooms, code, 1600)
			if tc.close {
				if _, err := rooms.transition(code, roomOpen, roomClosed); err != nil {
					t.Fatal(err)
				}
			}
			request := httptest.NewRequest(http.MethodPost, "/rooms/"+code+"/participants", nil)
			request.SetPathValue("code", code)
			rec := httptest.NewRecorder()

			joinHandler(rooms)(rec, request)

			if body := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusConflict || body != tc.want {
				t.Errorf("response = %d %q, want %d %q", rec.Code, body, http.StatusConflict, tc.want)
			}
		})
	}
}
