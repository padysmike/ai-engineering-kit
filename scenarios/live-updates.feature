Feature: Live updates
  Changes in a room reach the people viewing it without a reload.

  Scenario: A post appears live in the feed under the author's alias
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    When attendee A submits the post "Will the slides be shared?"
    Then attendee B sees the post "Will the slides be shared?" in the feed without reloading
    And the post is shown with the alias of attendee A

  Scenario: Participant count updates live
    Given an open room
    And attendee A has joined the room
    And the host sees a participant count of 1
    When attendee B joins the room
    Then the host sees a participant count of 2 without reloading

  Scenario: The feed re-ranks live
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    And attendee C has joined the room
    And attendee A has submitted the post "Leading"
    And attendee A has submitted the post "Rising"
    And attendee C has upvoted the post "Leading"
    And attendee A is viewing the feed sorted by top
    When attendee C upvotes the post "Rising"
    And attendee B upvotes the post "Rising"
    Then the feed of attendee A lists "Rising", "Leading" in that order without reloading
