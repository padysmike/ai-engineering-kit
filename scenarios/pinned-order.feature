Feature: Pinned posts come first
  Pinned posts lead the feed in both orders. Among the pinned posts, and
  among the others, the chosen order applies.

  Background:
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    And attendee C has joined the room

  Scenario: A pinned post leads the feed sorted by newest
    Given attendee A has submitted the post "Older"
    And attendee A has submitted the post "Newer" after the post "Older"
    And the host has pinned the post "Older"
    When attendee B sorts the feed by newest
    Then the feed of attendee B lists "Older", "Newer" in that order

  Scenario: A pinned post leads the feed sorted by top
    Given attendee A has submitted the post "One vote"
    And attendee A has submitted the post "No votes"
    And attendee C has upvoted the post "One vote"
    And the host has pinned the post "No votes"
    When attendee B sorts the feed by top
    Then the feed of attendee B lists "No votes", "One vote" in that order

  Scenario: Pinned posts are sorted among themselves
    Given attendee A has submitted the post "Pinned without votes"
    And attendee A has submitted the post "Pinned with a vote"
    And attendee A has submitted the post "Not pinned"
    And attendee C has upvoted the post "Pinned with a vote"
    And attendee C has upvoted the post "Not pinned"
    And attendee B has upvoted the post "Not pinned"
    And the host has pinned the post "Pinned without votes"
    And the host has pinned the post "Pinned with a vote"
    When attendee B sorts the feed by top
    Then the feed of attendee B lists "Pinned with a vote", "Pinned without votes", "Not pinned" in that order
