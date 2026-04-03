import requests

INGRES_API_URL = "https://ingres.iith.ac.in/api/gec/getBusinessDataForUserOpen"

def fetch_raw_data(location: str, year: str, view: str, locuuid: str) -> dict:
    """
    Fetches raw data from the INGRES API.
    """
    payload = {
        "parentLocName": "INDIA",
        "locname": "INDIA",
        "loctype": "COUNTRY",
        "view": view,
        "locuuid": locuuid,
        "year": year,
        "computationType": "normal",
        "component": "recharge",
        "period": "annual",
        "category": "safe",
        "mapOnClickParams": "false",
        "verificationStatus": 1,
        "approvalLevel": 1,
        "parentuuid": "ffce954d-24e1-494b-ba7e-0931d8ad6085",
        "stateuuid": None
    }
    
    response = requests.post(INGRES_API_URL, json=payload)
    response.raise_for_status()
    data = response.json()
    
    # Simple search for testing the plumbing, matching Go logic slightly if needed
    # The payload searches by locuuid anyway, but we just return the first hit or full data.
    if data and isinstance(data, list) and len(data) > 0:
        return {"records": len(data), "sample": data[0]}
    
    return {"records": 0, "sample": {}}
