Feature: Post marks
  The host pins posts and marks them as answered.

  Background:
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    And attendee C has joined the room

  Scenario: Host pins a post
    Given attendee A has submitted the post "Question one"
    When the host pins the post "Question one"
    Then attendee B sees the post "Question one" marked as pinned without reloading

  Scenario: Host marks a post as answered
    Given attendee A has submitted the post "Question one"
    When the host marks the post "Question one" as answered
    Then attendee B sees the post "Question one" marked as answered without reloading
