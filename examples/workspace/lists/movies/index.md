---
name: Movies
description: Films that I watched or want to watch.
schema:
  type: object
  required: [title, director, imdb, rotten_tomatoes]
  properties:
    title: {type: string}
    director: {type: string}
    imdb: {type: number}
    rotten_tomatoes: {type: number}
    cast:
      type: array
      items: {type: string}
---

# Movies

Films that I watched or want to watch.
