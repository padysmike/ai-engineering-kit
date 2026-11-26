Feature: Post limits
  A post has a maximum length, and a closed room accepts no new posts.

  # Not covered by scenarios:
  # - Per-device rate limit: the number of posts and the time window are not
  #   decided.
  # - Profanity filter: the word list and what happens to a matching post are
  #   not decided.

  Scenario: A post of 280 characters is accepted
    Given an open room
    And attendee A has joined the room
    When attendee A submits a post of 280 characters
    Then the feed contains that post

  Scenario: A post of 281 characters is rejected
    Given an open room
    And attendee A has joined the room
    When attendee A submits a post of 281 characters
    Then the post is rejected
    And the feed contains 0 posts

  Scenario: A closed room is read-only for posts
    Given an open room
    And attendee A has joined the room
    And attendee A has submitted the post "Asked while open"
    And the host has closed the room
    When attendee A submits the post "Asked after closing"
    Then the post is rejected
    And the feed of attendee A contains only the post "Asked while open"
