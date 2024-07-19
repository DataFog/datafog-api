FROM ubuntu:22.04
ENV PYTHONUNBUFFERED=1
ENV DEBIAN_FRONTEND=noninteractive

EXPOSE 8000

RUN apt-get update && apt-get install -y \
    vim \
    git \
    python3-pip \
    python3.11 \
    wget

ADD app /root/app

RUN python3.11 -m pip install -r /root/app/requirements.txt


WORKDIR /root/app
ENTRYPOINT ["python3.11", "-m", "uvicorn", "--host=0.0.0.0","main:app"]