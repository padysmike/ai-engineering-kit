Feature: Join by room code
  An attendee joins a room with its 6-character room code and gets an
  anonymous alias.

  # Not covered by scenarios:
  # - Randomness of aliases: a few samples cannot show that a value is random.

  Scenario: Joining with the 6-character room code
    Given an open room
    When a new attendee enters the room's 6-character room code
    Then the attendee is in the room
    And the attendee is signed in with an anonymous alias
    And the join request contained no password or email

  Scenario: Each room has its own participants
    Given 2 open rooms
    When attendee A joins the first room
    Then the participant count of the first room is 1
    And the participant count of the second room is 0
