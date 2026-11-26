Feature: QR login
  An attendee joins a room by scanning the QR code on the host's screen,
  without installing anything or filling in a sign-up form.

  # Not covered by scenarios:
  # - "Works on any mobile browser": the set of all mobile browsers cannot be
  #   exercised by an automated scenario. Check it manually on real devices.
  # - Opening the phone's browser after a scan is done by the phone itself. The
  #   scenarios start when the URL from the QR code is requested.

  Scenario: The QR code encodes the room URL with a token
    Given an open room
    When the host displays the room's QR code
    Then the QR code encodes a URL of that room
    And the URL contains a token

  Scenario: Scanning the QR code signs the attendee in
    Given an open room
    When a new attendee opens the URL from the room's QR code
    Then the attendee is in the room
    And the attendee is signed in with an anonymous alias
    And the join request contained no password or email
