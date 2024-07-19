"""Unit tests for processor.py"""
from processor import format_pii_for_output, find_pii_in_text, anonymize_pii_for_output


def test_format_pii_for_output():
    data = {
        "My name is Peter Parker. I live in Queens, NYC. I work at the Daily Bugle.": {
            "DATE_TIME": [],
            "LOC": ["Queens", "NYC"],
            "NRP": [],
            "ORG": ["the Daily Bugle"],
            "PER": ["Peter Parker"],
        }
    }
    res = format_pii_for_output(data)
    assert (
        res["entities"][0]["text"] == "Peter Parker"
    ), "The first entity's text is not 'Peter Parker'."


def test_find_pii_in_text_duplicate_pii_of_different_type():
    original_text = "Kaladin works for Apple on the main Apple campus"
    start_index = 0
    pii = "Apple"
    seen = set()
    seen.add(18)
    result = find_pii_in_text(original_text, start_index, pii, seen)
    assert(result[0] == 36), "found pii does not start at correct location"
    assert(result[1] == 41), "found pii does not end at correct location"


def test_find_pii_in_text_prefix_to_ignore():
    original_text = "the samovar belongs to sam"
    start_index = 0
    pii = "sam"
    seen = set()
    result = find_pii_in_text(original_text, start_index, pii, seen)
    assert(result[0] == 23), "found pii does not start at correct location"
    assert(result[1] == 26), "found pii does not end at correct location"


def test_find_pii_in_text_suffix_to_ignore():
    original_text = "He stopped ed from jumping"
    start_index = 0
    pii = "ed"
    seen = set()
    result = find_pii_in_text(original_text, start_index, pii, seen)
    assert(result[0] == 11), "found pii does not start at correct location"
    assert(result[1] == 13), "found pii does not end at correct location"


def test_anonymize_pii():
    data = {
        "My name is Peter Parker. I live in Queens, NYC. I work at the Daily Bugle.": {
            "DATE_TIME": [],
            "LOC": ["Queens", "NYC"],
            "NRP": [],
            "ORG": ["the Daily Bugle"],
            "PER": ["Peter Parker"],
        }
    }
    out = anonymize_pii_for_output(data)
    assert(
        out["text"] == "My name is [PER]. I live in [LOC], [LOC]. I work at [ORG]."
    ), "text anonymized incorrectly"
