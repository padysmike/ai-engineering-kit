---
name: scaffold-project
description: Create the project skeleton (backend, minimal frontend, Makefile, scripts, agent instructions) in an empty repository. Use once at the start of a new product.
license: MIT
metadata:
  invocation: manual
---

# Scaffold the project

1. Confirm the target directory is empty or a fresh git repository.
2. Run `sh skills/scaffold-project/scripts/scaffold.sh <target-dir> <module-name>`.
3. In the target directory run `make check` and report the result.
4. Do not commit. Show the human what was created.
