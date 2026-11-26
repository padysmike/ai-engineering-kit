Feature: Voting
  Attendees upvote posts.

  Background:
    Given an open room
    And attendee A has joined the room
    And attendee B has joined the room
    And attendee C has joined the room

  Scenario: Attendee upvotes a post
    Given attendee A has submitted the post "Question one"
    When attendee B upvotes the post "Question one"
    Then the post "Question one" has 1 vote

  Scenario: Upvoting the same post again removes the vote
    Given attendee A has submitted the post "Question one"
    And attendee B has upvoted the post "Question one"
    When attendee B upvotes the post "Question one"
    Then the post "Question one" has 0 votes

  Scenario: Votes are counted per post
    Given attendee A has submitted the post "Question one"
    And attendee A has submitted the post "Question two"
    When attendee C upvotes the post "Question one"
    And attendee B upvotes the post "Question one"
    And attendee B upvotes the post "Question two"
    Then the post "Question one" has 2 votes
    And the post "Question two" has 1 vote
