#!/usr/bin/env star

# Function to check if an image is exempted
def enrich(input):
    data = input.payload
    Severity01Critical = 0
    Severity02High = 0
    Severity03Medium = 0
    Severity04Low = 0
    Severity05Negligible = 0
    for finding in data.Image.Findings:
        if finding.Severity == "Critical":
            Severity01Critical += 1
        elif finding.Severity == "High":
            Severity02High += 1
        elif finding.Severity == "Medium":
            Severity03Medium += 1
        elif finding.Severity == "Low":
            Severity04Low += 1
        elif finding.Severity == "Negligible":
            Severity05Negligible += 1

    return { 
        "Severity01Critical": Severity01Critical,
        "Severity02High": Severity02High,
        "Severity03Medium": Severity03Medium,
        "Severity04Low": Severity04Low,
        "Severity05Negligible": Severity05Negligible
        }

