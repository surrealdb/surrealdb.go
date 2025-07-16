# Potential Breaking Changes

This document is supposed to list up findings on and feedbacks to the SDK v0.x that might result in breaking changes if addressed.

Depending on the degree of "breaking", we might address those while in v0, or when cutting v1.

- `NewGeometryPoint` takes the latitude and the longtitude in an order that conflicts with SurrealDB. Based on GeoJSON, SurrealDB assumes the longtitude appear earlier, while `NewGeometryPoint` takes the latitude earlier. TThe function should be changed to take the longtitude earlier. Related issue: #223
