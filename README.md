# JetStore

JetStore is a compute analytic platform for cloud-native data processing on AWS. It loads client
files, maps them to a domain model, applies rules with a high-performance C++ RETE engine, and runs
analytical pipelines (compute pipes) that scale out across Lambda functions and ECS tasks, writing
the results to PostgreSQL and S3. The orchestration and the API server are written in Go, the web
application in React and TypeScript, and the infrastructure is deployed with the AWS CDK. What an
installation does for a client is defined by a workspace: its rules, pipeline configurations,
reports and data model, compiled into the images JetStore deploys.

## Documentation

The documentation is in [`docs/`](docs/): the User Guide, the Developer's Guide and the Installation
Guide, built into one searchable site with Astro Starlight. An installation serves the site for the
version it runs at `/docs/`, opened from the *Help* button in JetStore's top bar. To preview it from
a checkout, run `npm ci && npm run dev` in `docs/`; [writing the documentation](docs/src/content/docs/developer/contributing/documentation.md)
explains how pages are organised and checked.
