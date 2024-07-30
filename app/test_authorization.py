"""Unit tests for authorization.py"""

# Standard library imports
from unittest.mock import patch

# Third party imports
from fastapi.security import HTTPBasicCredentials

# Local imports
from authorization import is_valid_basic_request

TEST_USER = "jsmith"
TEST_PASSWORD = "1234"


@patch("authorization.load_valid_credentials")
def test_is_valid_request_true(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = (TEST_USER.encode("utf8"), TEST_PASSWORD.encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_basic_request(credentials)

    # Assert that the result is as expected
    assert result
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


@patch("authorization.load_valid_credentials")
def test_is_valid_request_false_user(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = ("wrong_user".encode("utf8"), TEST_PASSWORD.encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_basic_request(credentials)

    # Assert that the result is as expected
    assert result is False
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


@patch("authorization.load_valid_credentials")
def test_is_valid_request_false_password(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = (TEST_USER.encode("utf8"), "wrong_password".encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_basic_request(credentials)

    # Assert that the result is as expected
    assert result is False
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()


@patch("authorization.load_valid_credentials")
def test_is_valid_request_false_both(mock_load_valid_credentials):
    # Define the mock and test data
    mock_data = ("wrong_user".encode("utf8"), "wrong_password".encode("utf8"))
    mock_load_valid_credentials.return_value = mock_data
    credentials = HTTPBasicCredentials(username=TEST_USER, password=TEST_PASSWORD)

    # Call the function
    result = is_valid_basic_request(credentials)

    # Assert that the result is as expected
    assert result is False
    # Ensure that load_valid_credentials was called
    mock_load_valid_credentials.assert_called_once()
