## AWS Lambda Telemetry Extension

The AWS Lambda Telemetry Extension is an AWS Lambda Extension that collects lambda metrics, traces, and logs asynchronously while your AWS Lambda function executes and forwards to fluentd, s3 etc.

## What are AWS Lambda Extensions?
Lambda extensions are shared libraries that run side-by-side with functions inside the same execution environment.
1. [AWS Lambda Extensions](https://aws.amazon.com/blogs/compute/introducing-aws-lambda-extensions-in-preview/)
2. [Extension API](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-extensions-api.html)
3. [Telemetry API](https://docs.aws.amazon.com/lambda/latest/dg/telemetry-api.html)

## Usage:

Add the layer to your lambda ([Working with lambda layers](https://docs.aws.amazon.com/lambda/latest/dg/chapter-layers.html#configuration-layers-using)).

## Configuration:

Use the following environment variables to enable and control the extension behavior.

| Name | Value                                                                                                                                |
    |----- |--------------------------------------------------------------------------------------------------------------------------------------|
| `TELEMETRY_EXTENSION_ENABLED`| Set `TELEMETRY_EXTENSION_ENABLED` to `true` to enable the extension. (Required)                                                      |
| `TELEMETRY_EXTENSION_SUBSCRIBE_EVENTS`| Comma separated TelemetryAPI subscribe events. Supported events: `function`, `platform`, `extension`. (Default: `function`)          |
| `TELEMETRY_EXTENSION_FLUENTD_ENABLED` | Forwards the telemetry info to Fluentd. (Default: `false`)                                                                           |
| `TELEMETRY_EXTENSION_FLUENTD_HOST` | Fluentd Host (Used when `TELEMETRY_EXTENSION_FLUENTD_ENABLED` `true`).                                                                        |
| `TELEMETRY_EXTENSION_FLUENTD_PORT` | Fluentd Port (Used when `TELEMETRY_EXTENSION_FLUENTD_ENABLED` `true`). (Default: `24224`)                                                     |
| `TELEMETRY_EXTENSION_FLUENTD_TAG_NAME` | FluentD Tag name. (Default: `lambda`)                                                                                                |

