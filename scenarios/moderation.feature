Feature: Moderation
  The host manages posts and participants.

  Scenario: Host hides a post
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    And attendee A has submitted the post "Off topic"
    When the host hides the post "Off topic"
    Then the feed of attendee B does not contain the post "Off topic"

  Scenario: Host deletes a post
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    And attendee A has submitted the post "Off topic"
    When the host deletes the post "Off topic"
    Then the feed of attendee B does not contain the post "Off topic"
    And the feed of the host does not contain the post "Off topic"

  Scenario: Host kicks a participant
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    When the host kicks attendee A
    Then the participant count of the room is 1
    And attendee A no longer has access to the room
    And attendee B still has access to the room
