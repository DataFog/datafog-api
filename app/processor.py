"""Collection of functional hooks that leverage specialized classes"""
from constants import ResponseKeys

def format_pii_for_output(pii: dict[str, dict]) -> dict:
    """Reformat datafog library results to meet API contract"""
    sorted_entities = get_entities_from_pii(pii)
    return {ResponseKeys.TITLE.value: sorted_entities}

def get_entities_from_pii(pii: dict[str, dict]) -> list:
    """Produce a sorted list of entities from the datafog library results"""
    entities = []                               #list of entities to output
    claimed_start_indices = set()               #Set of start indices of PII that have been found
    original_text = list(pii.keys())[0]         #original text fed to datafog library
    dict_of_pii_types = pii[original_text]      #dict of PII entities keyed by type
    for k,v in dict_of_pii_types.items():
        #loop through each PII entity type and add the found entities to the output list
        #must use extend to add each entity from the returned list, append would add the whole list
        entities.extend(create_entities(original_text, k, v, claimed_start_indices))
    #sort entities by the start index of the PII in the original text and add to the output dict
    return sorted(entities, key=lambda d: d[ResponseKeys.START_IDX.value])

def create_entities(original_text: str, pii_type: str, pii_list, seen_indices: set) -> list:
    """Create an output list of PII entities from a list of PII of a particular type"""
    result = []
    start_index = 0
    for pii in pii_list:
        #for each pii in the input list find it in the original text and create an response
        # entity to add to the output list
        result.append(create_entity(original_text, start_index, pii_type, pii, seen_indices))
        #begin the search for the next PII at the next character after the end of the PII
        # just added to the output by updating startIndex
        start_index = result[-1][ResponseKeys.END_IDX.value] + 1
    return result

def create_entity(original_text: str,
                 start_index: int,
                 pii_type: str,
                 pii: str,
                 seen_indices: set) -> dict:
    """Create an output PII entity from a singular datafog library result"""
    #TODO: fail gracefully if we cant find the pii in the original text
    start, end = find_pii_in_text(original_text, start_index, pii, seen_indices)
    result = {ResponseKeys.PII_TEXT.value: pii,
              ResponseKeys.START_IDX.value: start,
              ResponseKeys.END_IDX.value: end,
              ResponseKeys.ENTITY_TYPE.value: pii_type}
    return result

def find_pii_in_text(original_text: str,
                  start_index: int,
                  pii: str,
                  seen_indices: set) -> tuple[int, int]:
    """Find pii in the original text and return the start and end index"""
    start = None
    end = None
    #Currently returns the 0-based start index and the end index non-inclusive
    while start_index < len(original_text):
        start = original_text.find(pii, start_index)
        if start == -1:
            #unable to find PII, return None
            return (None, None)

        end = start + len(pii)

        if start > 0 and original_text[start - 1 : start].isalnum():
            #if the start is not the first char and the char before it is an alpha numeric
            # we have found a substring and should continue
            start_index = start + 1
            #continue search after start
            start = None
            end = None
            continue
        elif end < len(original_text) and original_text[end : end + 1].isalnum():
            #if end is not the last char in the text and the char after it is an alpha numeric
            # we have found a substring and should continue
            start_index = start + 1
            #continue search after start
            start = None
            end = None
            continue
        elif start in seen_indices:
            # we have previously found this pii and should continue
            start_index = start + 1
            #continue search after start
            start = None
            end = None
            continue

        #Valid PII found, break out of loop
        break

    #add start to the set of seen indices
    seen_indices.add(start)
    return (start, end)

def anonymize_pii_for_output(pii: dict[str, dict]) -> dict:
    """Given datafog library results uses helper functions to anonymize and return the text"""
    original_text = list(pii.keys())[0]         #original text fed to datafog library
    entities = get_entities_from_pii(pii)
    anonymized_text = anonymize_pii_in_text(entities, original_text)
    response = {ResponseKeys.PII_TEXT.value: anonymized_text,
                ResponseKeys.TITLE.value: entities}
    return response

def anonymize_pii_in_text(pii_entities: list, text: str) -> str:
    """Anonymize the provided entities in the text"""
    offset = 0                                  #track the changes in length of the text
    original_text_length = len(text)
    for ent in pii_entities:
        place_holder = "[" + ent[ResponseKeys.ENTITY_TYPE.value] + "]"
        #calculate the new start and stop indices to account for the updates so far
        start = ent[ResponseKeys.START_IDX.value] - offset
        stop = ent[ResponseKeys.END_IDX.value] - offset
        #substitute into text subtracting offset
        text = text[:start] + place_holder + text[stop:]
        #update offset to account for the new string
        offset = original_text_length - len(text)

    return text
