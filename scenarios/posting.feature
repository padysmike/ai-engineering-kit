Feature: Posting
  Attendees submit questions and comments without their identity being shown
  to anyone, including the host.

  Scenario: A post appears in the feed under the author's alias
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    When attendee A submits the post "Will the slides be shared?"
    Then attendee B sees the post "Will the slides be shared?" in the feed
    And the post is shown with the alias of attendee A

  Scenario: The host sees only the alias of an author
    Given an open room
    And attendee A has joined the room
    When attendee A submits the post "Will the slides be shared?"
    Then the host sees the post "Will the slides be shared?" with the alias of attendee A
    And the data sent to the host does not contain the session token of attendee A

  Scenario: The alias stays the same within a room
    Given an open room
    And attendee A has joined the room
    When attendee A submits the post "First question"
    And attendee A submits the post "Second question"
    Then both posts are shown with the alias attendee A received on joining
