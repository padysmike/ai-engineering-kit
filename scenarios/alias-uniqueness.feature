Feature: Alias uniqueness
  No two attendees in a room share an alias.

  Scenario: 50 attendees in one room get different aliases
    Given an open room
    When 50 attendees join the room
    Then all 50 aliases are different
