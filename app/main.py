"""API REST endpoints"""
from fastapi import FastAPI, Body, Request
from fastapi.exceptions import RequestValidationError
from datafog import DataFog
from processor import format_pii_for_output, anonymize_pii_for_output
from constants import VALID_CHARACTERS_PATTERN
from exception_handler import exception_processor
from input_validation import validate_annotate, validate_anonymize

app = FastAPI()
df = DataFog()

@app.post("/api/annotation/default")
def annotate(text: str = Body(embed=True,
                              min_length=1,
                              max_length=1000,
                              pattern=VALID_CHARACTERS_PATTERN),
             lang: str = Body(embed=True,
                              default="EN")):
    """entry point for annotate functionality"""
    #Use the custom validation imported above, currently only lang requires custom validation
    validate_annotate(lang)
    result = df.run_text_pipeline_sync([text])
    output = format_pii_for_output(result)
    return output

@app.post("/api/anonymize/non-reversible")
def anonymize(text: str = Body(embed=True,
                               min_length=1,
                               max_length=1000,
                               pattern=VALID_CHARACTERS_PATTERN),
              lang: str = Body(embed=True, default="EN")):
    """entry point for anonymize functionality"""
    #Use the custom validation imported above, currently only lang requires custom validation
    validate_anonymize(lang)
    result = df.run_text_pipeline_sync([text])
    output = anonymize_pii_for_output(result)
    return output

@app.exception_handler(RequestValidationError)
async def validation_exception_handler(request: Request, exc: RequestValidationError):
    """exception handling hook for input validation failures"""
    #offload actual processing to another to keep this uncluttered
    return exception_processor(request, exc)
