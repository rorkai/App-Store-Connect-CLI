# CI/CD Integrations

## GitHub Actions

Install `asc` using the official setup action:

```yaml
- uses: rudrankriyam/setup-asc@v1
  with:
    version: latest

- run: asc --help
```

For end-to-end examples, see:
https://github.com/rudrankriyam/setup-asc

## GitLab CI/CD Components

Use the official `asc-ci-components` repository:

```yaml
include:
  - component: gitlab.com/rudrankriyam/asc-ci-components/run@main
    inputs:
      stage: deploy
      job_prefix: release
      asc_version: latest
      command: asc --help
```

For install/run templates and self-managed examples:
https://github.com/rudrankriyam/asc-ci-components

## Bitrise

Use the official `setup-asc` Bitrise step repository:

```yaml
workflows:
  primary:
    steps:
    - git::https://github.com/rudrankriyam/steps-setup-asc.git@main:
        inputs:
        - mode: run
        - version: latest
        - command: asc --help
```

## CircleCI

Use the official CircleCI orb repository:
https://github.com/rudrankriyam/asc-orb
