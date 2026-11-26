Feature: Feed sorting
  The feed sorts by votes, so that the top questions surface, or by newest.

  Background:
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    And attendee C has joined the room

  Scenario: Feed sorted by top
    Given attendee A has submitted the post "One vote"
    And attendee A has submitted the post "Two votes"
    And attendee A has submitted the post "No votes"
    And attendee C has upvoted the post "Two votes"
    And attendee B has upvoted the post "Two votes"
    And attendee B has upvoted the post "One vote"
    When attendee B sorts the feed by top
    Then the feed of attendee B lists "Two votes", "One vote", "No votes" in that order

  Scenario: Feed sorted by newest
    Given attendee A has submitted the post "Older"
    And attendee A has submitted the post "Newer" after the post "Older"
    And attendee B has upvoted the post "Older"
    When attendee B sorts the feed by newest
    Then the feed of attendee B lists "Newer", "Older" in that order
