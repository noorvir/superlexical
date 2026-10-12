---
name: Historical Figures
description: People from history that I want to remember.
schema:
  type: object
  required: [name, role]
  properties:
    name: {type: string}
    role: {type: string}
    born: {type: string}
    died: {type: string}
    places:
      type: array
      items: {type: string}
---

# Historical Figures

People from history that I want to remember.
