<p align="center">
  <a href="https://www.datafog.ai"><img src="https://github.com/DataFog/datafog-python/raw/main/public/colorlogo.png" alt="DataFog logo"></a>
</p>

<p align="center">
    <b>REST API for PII detection and anonymization</b>. <br />
</p>

<p align="center">

<a href="https://hub.docker.com/r/datafog/datafog-api"><img src="https://img.shields.io/badge/Docker-2CA5E0?style=for-the-badge&logo=docker&logoColor=white" alt="Docker"></a>

<a href="https://discord.gg/bzDth394R4"><img src="https://img.shields.io/discord/1173803135341449227?style=flat" alt="Discord"></a>

<a href="https://github.com/psf/black"><img src="https://img.shields.io/badge/code%20style-black-000000.svg?style=flat-square" alt="Code style: black"></a>

  <!-- <a href="https://codecov.io/gh/datafog/datafog-api"><img src="https://img.shields.io/codecov/c/github/datafog/datafog-api.svg?style=flat-square" alt="codecov"></a> -->

## Overview

datafog-api is a REST API service that lets you detect ('annotate'), and anonymize sensitive information in your data using specialized ML models.

## Getting Started:

### Option 1: Install via Docker (fastest)

The fastest way to get started with datafog-api is to pull the image from Docker:

```sh
docker pull datafog/datafog-api:latest
```

### Option 2: Build from source

Alternatively, you can clone this repository and build the image locally:

```sh
git clone https://github.com/datafog/datafog-api.git
```

```sh
cd datafog-api/
docker build -t datafog-api .
```

### Then, run the Docker container

```sh
docker run -p 8000:8000  -it datafog-api
```

> **NOTE** Change the first 8000 to a new port if there is a conflict.

## Example cURL request/responses

### Annotation

Request:

```sh
curl -X POST http://127.0.0.1:80/api/annotation/default \
     -H "Content-Type: application/json" \
     -d '{"text": "My name is Peter Parker. I live in Queens, NYC. I work at the Daily Bugle."}'
```

Response:

```sh
{
  "entities": [
    {"text": "Peter Parker", "start": 11, "end": 23, "type": "PER"},
    {"text": "Queens", "start": 35, "end": 41, "type": "LOC"},
    {"text": "NYC", "start": 43, "end": 46, "type": "LOC"},
    {"text": "the Daily Bugle", "start": 58, "end": 73, "type": "ORG"}
  ]
}
```

### Anonymization (One-way)

Request:

```sh
curl -X POST http://0.0.0.0:80/api/anonymize/non-reversible \
     -H "Content-Type: application/json" \
     -d '{"text": "My name is Peter Parker. I live in Queens, NYC. I work at the Daily Bugle."}'
```

Response:

```sh
{
  "text": "My name is [PER]. I live in [LOC], [LOC]. I work at [ORG].",
  "entities": [
    {"text": "Peter Parker", "start": 11, "end": 23, "type": "PER"},
    {"text": "Queens", "start": 35, "end": 41, "type": "LOC"},
    {"text": "NYC", "start": 43, "end": 46, "type": "LOC"},
    {"text": "the Daily Bugle", "start": 58, "end": 73, "type": "ORG"}
  ]
}
```

### Anonymization (Reversible)

Coming soon!

## Advanced

### Local Development

```sh
git clone https://github.com/datafog/datafog-api.git
cd datafog-api/
python -m venv myenv
source myenv/bin/activate
cd app
pip install -r requirements.txt
uvicorn main:app
```

> **NOTE** datafog-api requires Python 3.11+

## Contributing

DataFog is a community-driven **open-source** platform and we've been fortunate to have a small and growing contributor base. We'd love to hear ideas, feedback, suggestions for improvement - anything on your mind about what you think can be done to make DataFog better! Join our [Discord](https://discord.gg/bzDth394R4) and be a part of our growing community.

### Contributors

- sroy9675
- pselvana
- sidmohan0

# License

This software is published under the [MIT
license](https://en.wikipedia.org/wiki/MIT_License).
