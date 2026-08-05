# Agent instructions

## Infrastructure

Deploy inventory for this project lives in the sibling repo `rootfox.cc-infra` at `state/dplo/projects/mogotor/`.

When deploy paths, ports, env, secrets refs, nginx, or dplo scripts change, update `rootfox.cc-infra` in the same change set and keep it current.

## Deploy

Build/restart script lives in dplo project scripts (`rootfox.cc-infra` / `/var/lib/dplo/projects/mogotor/scripts/build.sh`), not in this repo.
