#!/usr/bin/env star

# Return summary of findings
def enrich(input):
    data = input.payload
    severity_01_critical = 0
    severity_02_high = 0
    severity_03_medium = 0
    severity_04_low = 0
    severity_05_negligible = 0
    if data.Image.Findings != None:
        for finding in data.Image.Findings:
            if finding.Severity == "Critical":
                severity_01_critical += 1
            elif finding.Severity == "High":
                severity_02_high += 1
            elif finding.Severity == "Medium":
                severity_03_medium += 1
            elif finding.Severity == "Low":
                severity_04_low += 1
            elif finding.Severity == "Negligible":
                severity_05_negligible += 1
    alerted = False
    if severity_01_critical > 0 or severity_02_high > 0:
        alerted = True
    return { 
        "Severity01Critical": severity_01_critical,
        "Severity02High": severity_02_high,
        "Severity03Medium": severity_03_medium,
        "Severity04Low": severity_04_low,
        "Severity05Negligible": severity_05_negligible,
        "Alerted": alerted
    }

