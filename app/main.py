"""API REST endpoints"""

import json
import os
import secrets

from datafog import DataFog
from fastapi import Body, Depends, FastAPI, HTTPException, Request, status
from fastapi.exceptions import RequestValidationError
from fastapi.security import HTTPBasic, HTTPBasicCredentials

from constants import AUTH_ENABLED_KEY, PASSWORD_KEY, USER_KEY, VALID_INPUT_PATTERN
from exception_handler import exception_processor
from input_validation import validate_annotate, validate_anonymize
from processor import (
    anonymize_pii_for_output,
    encode_pii_for_output,
    format_pii_for_output,
)

app = FastAPI()
security = HTTPBasic()
df = DataFog()

AUTH_ENABLED = os.getenv(AUTH_ENABLED_KEY, "false").lower() == "true"


@app.post("/api/annotation/default")
def annotate(
    text: str = Body(embed=True, min_length=1, max_length=1000, pattern=VALID_INPUT_PATTERN),
    lang: str = Body(embed=True, default="EN"),
):
    """entry point for annotate functionality"""
    # Use the custom validation imported above, currently only lang requires custom validation
    validate_annotate(lang)
    result = df.run_text_pipeline_sync([text])
    output = format_pii_for_output(result)
    return output


@app.post("/api/anonymize/non-reversible")
def anonymize(
    text: str = Body(embed=True, min_length=1, max_length=1000, pattern=VALID_INPUT_PATTERN),
    lang: str = Body(embed=True, default="EN"),
):
    """entry point for anonymize functionality"""
    # Use the custom validation imported above, currently only lang requires custom validation
    validate_anonymize(lang)
    result = df.run_text_pipeline_sync([text])
    output = anonymize_pii_for_output(result)
    return output


def get_authorization(credentials: HTTPBasicCredentials = Depends(security)):
    try:
        authorized_user_bytes = os.environ[USER_KEY].encode("utf8")
        password_bytes = os.environ[PASSWORD_KEY].encode("utf8")
    except KeyError:
        # auth config not set in env variables, attempt to read from .env file
        authorized_user_bytes, password_bytes = load_credentials_file()
    current_username_bytes = credentials.username.encode("utf8")
    is_correct_username = secrets.compare_digest(
        current_username_bytes, authorized_user_bytes
    )
    current_password_bytes = credentials.password.encode("utf8")
    is_correct_password = secrets.compare_digest(
        current_password_bytes, password_bytes
    )

    if not (is_correct_username and is_correct_password):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Incorrect username or password",
            headers={"WWW-Authenticate": "Basic"},
        )
    return credentials.username


@app.post("/api/anonymize/reversible")
def encode(
    text: str = Body(embed=True, min_length=1, max_length=1000, pattern=VALID_INPUT_PATTERN),
    lang: str = Body(embed=True, default= "EN"),
    salt: str = Body(embed=True, min_length=16, max_length=64),
    user: str = Depends(get_authorization),
):
    """entry point for reversible anonymize functionality"""
    if AUTH_ENABLED:
        print(f"Verified user: {user}")
    # Use the custom validation imported above, currently only lang requires custom validation
    validate_anonymize(lang)
    result = df.run_text_pipeline_sync([text])
    output = encode_pii_for_output(result, salt)
    return output


@app.exception_handler(RequestValidationError)
async def validation_exception_handler(request: Request, exc: RequestValidationError):
    """exception handling hook for input validation failures"""
    # offload actual processing to another to keep this uncluttered
    return exception_processor(request, exc)


def load_credentials_file() -> tuple[bytes, bytes]:
    """attempt to read authentication configuration from file, throw if unsuccessful"""

    with open(".env", "r") as auth_config:
        config_dict = json.load(auth_config)

    try:
        authorized_user_bytes = config_dict[USER_KEY].encode("utf8")
        password_bytes = config_dict[PASSWORD_KEY].encode("utf8")
    except KeyError as exc:
        #failed to find in file, throwing
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Authorization configuration is not complete, please add authorized Users",
            headers={"WWW-Authenticate": "Basic"},
        ) from exc

    return (authorized_user_bytes, password_bytes)
