"""API REST endpoints"""

from constants import VALID_INPUT_PATTERN
from datafog import DataFog
from exception_handler import exception_processor
from fastapi import Body, FastAPI, Request
from fastapi.exceptions import RequestValidationError
from input_validation import validate_annotate, validate_anonymize
from processor import anonymize_pii_for_output, encode_pii_for_output, format_pii_for_output

app = FastAPI()
df = DataFog()


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


@app.post("/api/anonymize/reversible")
def encode(
    text: str = Body(embed=True, min_length=1, max_length=1000, pattern=VALID_INPUT_PATTERN),
    lang: str = Body(embed=True, default="EN"),
    salt: str = Body(embed=True, min_length=16, max_length=64),
):
    """entry point for reversible anonymize functionality"""
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
