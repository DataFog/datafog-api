"""Exception handling routines"""

from fastapi import Request, status
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse


def exception_processor(request: Request, exc: RequestValidationError):
    """Provide the opportunity for custom handling of standard fastapi errors if required"""
    for e in exc.errors():
        # switch on e["type"] if more standard fastapi 422 errors need to be altered
        # custom exceptions should manage output formatting during creation not here
        if e["type"] == "value_error.str.regex":
            e["msg"] = (
                "string contains unsupported characters beyond the Extended ASCII set"
            )
            e["ctx"]["pattern"] = "Extended ASCII"

    return JSONResponse(
        status_code=status.HTTP_422_UNPROCESSABLE_ENTITY,
        content={"detail": exc.errors()},
    )
