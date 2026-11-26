Feature: Voting in a closed room
  A closed room is read-only for votes.

  Background:
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    And attendee C has joined the room

  Scenario: A closed room does not accept a new vote
    Given attendee A has submitted the post "Question one"
    And the host has closed the room
    When attendee B upvotes the post "Question one"
    Then the vote is rejected
    And the post "Question one" has 0 votes

  Scenario: A closed room does not allow removing a vote
    Given attendee A has submitted the post "Question one"
    And attendee B has upvoted the post "Question one"
    And the host has closed the room
    When attendee B upvotes the post "Question one"
    Then the vote is rejected
    And the post "Question one" has 1 vote
