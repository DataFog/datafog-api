"""Custom input validation routines"""

from constants import SUPPORTED_LANGUAGES
from custom_exceptions import LanguageValidationError


def validate_annotate(lang: str):
    """Validation of annotate endpoint parameters not built into fastapi"""
    # currently only lang needs to be validated outside of standard fastapi checks
    validate_language(lang)


def validate_anonymize(lang: str):
    """Validation of anonymize endpoint parameters not built into fastapi"""
    # currently only lang needs to be validated outside of standard fastapi checks
    validate_language(lang)


def validate_language(lang: str):
    """Check that the input is in the list of languages supported by DataFog"""
    if lang not in SUPPORTED_LANGUAGES:
        raise LanguageValidationError(
            "Unsupported language request, please try a language listed in the DataFog documentation"
        )
