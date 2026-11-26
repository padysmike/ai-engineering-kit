Feature: Room lifecycle
  A room is one live session with its own feed, participants and settings.
  The host creates it and opens and closes it.

  # The read-only effect of a closed room on posts and votes is specified in
  # posting.feature and voting.feature.
  # Room settings have no scenarios: docs/product.md names no settings.

  Scenario: Each new room gets its own room code
    When the host creates 2 rooms
    Then each room has a 6-character room code
    And the 2 room codes are different

  Scenario: Host opens a draft room
    Given a draft room
    When the host opens the room
    Then the room state is "open"

  Scenario: Host closes an open room
    Given an open room
    When the host closes the room
    Then the room state is "closed"
