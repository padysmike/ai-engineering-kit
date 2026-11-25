Feature: Service health
  A monitoring system must be able to tell whether the service is alive.

  Scenario: Health endpoint reports ok
    When a client requests GET /healthz
    Then the response status is 200
    And the body contains "status":"ok"
