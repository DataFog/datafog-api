"""API REST endpoints"""

# Standard library imports
from typing import Optional

# Third party imports
from datafog import DataFog
from fastapi import Body, Depends, FastAPI, Request
from fastapi.exceptions import RequestValidationError

# Local imports
from authorization import AUTH_ENABLED, get_authorization
from constants import VALID_INPUT_PATTERN, AuthTypes
from exception_handler import exception_processor
from input_validation import validate_annotate, validate_anonymize
from processor import (
    anonymize_pii_for_output,
    encode_pii_for_output,
    format_pii_for_output,
)
from telemetry import telemetry_instance

app = FastAPI()
df = DataFog()
telemetry_instance.report_basic_telemetry()


@app.post("/api/annotation/default")
def annotate(
    text: str = Body(embed=True, min_length=1, max_length=1000, pattern=VALID_INPUT_PATTERN),
    lang: str = Body(embed=True, default="EN"),
    auth_type: Optional[AuthTypes] = Depends(get_authorization),
):
    """entry point for annotate functionality"""
    if AUTH_ENABLED:
        print(f"Verified authorization: {auth_type.value}")
    # Use the custom validation imported above, currently only lang requires custom validation
    validate_annotate(lang)
    result = df.run_text_pipeline_sync([text])
    output = format_pii_for_output(result)
    return output


@app.post("/api/anonymize/non-reversible")
def anonymize(
    text: str = Body(embed=True, min_length=1, max_length=1000, pattern=VALID_INPUT_PATTERN),
    lang: str = Body(embed=True, default="EN"),
    auth_type: Optional[AuthTypes] = Depends(get_authorization),
):
    """entry point for anonymize functionality"""
    if AUTH_ENABLED:
        print(f"Verified authorization: {auth_type.value}")
    # Use the custom validation imported above, currently only lang requires custom validation
    validate_anonymize(lang)
    result = df.run_text_pipeline_sync([text])
    output = anonymize_pii_for_output(result)
    return output


@app.post("/api/anonymize/reversible")
def encode(
    text: str = Body(embed=True, min_length=1, max_length=1000, pattern=VALID_INPUT_PATTERN),
    lang: str = Body(embed=True, default="EN"),
    salt: str = Body(embed=True, min_length=16, max_length=64),
    auth_type: Optional[AuthTypes] = Depends(get_authorization),
):
    """entry point for reversible anonymize functionality"""
    if AUTH_ENABLED:
        print(f"Verified authorization: {auth_type.value}")
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
