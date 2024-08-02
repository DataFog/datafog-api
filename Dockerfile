FROM ubuntu:23.10
ENV PYTHONUNBUFFERED=1
ENV DEBIAN_FRONTEND=noninteractive

EXPOSE 8000

RUN apt-get update && apt-get install -y \
    vim \
    git \
    python3.11 \
    python3-venv \
    wget

RUN useradd -ms /bin/bash datafoguser

ADD app /home/datafoguser/app

RUN python3.11 -m venv .venv && . .venv/bin/activate && \
    .venv/bin/pip install --upgrade pip && \
    .venv/bin/pip install -r /home/datafoguser/app/requirements.txt

WORKDIR /home/datafoguser/app
ENTRYPOINT ["sh", "docker-entrypoint.sh"]