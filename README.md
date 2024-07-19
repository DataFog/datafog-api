
# Local Development

> [!IMPORTANT]
> datafog-api requires Python 3.11+


## Setup
```sh
python -m venv myenv
source myenv/bin/activate
cd app
pip install requirements.txt
uvicorn main:app 
```


# Docker

## Build
```sh
docker build -t datafog-api .  
```

## Run
```sh
docker run -p 80:8000  -it datafog-api  
```
> [!TIP]
> Change 80 to a new port if there is a conflict.

## Test
```sh
curl -X POST http://127.0.0.1:80/api/annotation/default \
     -H "Content-Type: application/json" \
     -d '{"text": "My name is Peter Parker. I live in Queens, NYC. I work at the Daily Bugle."}'

{"entities":[{"text":"Peter Parker","start":11,"end":23,"type":"PER"},{"text":"Queens","start":35,"end":41,"type":"LOC"},{"text":"NYC","start":43,"end":46,"type":"LOC"},{"text":"the Daily Bugle","start":58,"end":73,"type":"ORG"}]}
```
