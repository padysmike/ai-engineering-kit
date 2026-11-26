Feature: QR tokens
  Expired or altered QR tokens do not sign anyone in, and the host can rotate
  the QR code.

  Scenario: An expired QR token does not sign the attendee in
    Given an open room
    And the token in the room's QR code has expired
    When a new attendee opens the URL from that QR code
    Then the attendee is not signed in
    And the participant count of the room is 0

  Scenario: An altered QR token does not sign the attendee in
    Given an open room
    When a new attendee opens the URL from the room's QR code with an altered token
    Then the attendee is not signed in
    And the participant count of the room is 0

  Scenario: Host rotates the QR code
    Given an open room with a displayed QR code
    When the host rotates the QR code
    Then the host's screen shows a QR code that differs from the previous one
    And scanning the new QR code opens the room
