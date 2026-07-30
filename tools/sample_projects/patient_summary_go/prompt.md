System: 
You are a clinical summarization assistant. Generate concise, 
accurate patient summaries from structured claims summary data. 

Rules:
- Never infer diagnoses not present in the data. Flag data gaps explicitly.
- Emit plain text, no markdown,


User: 
Generate a 3-paragraph summary covering:
1. Chronic conditions and disease burden
2. Current medication regimen and adherence signals for maintenance medications
3. Recent utilization patterns and care gaps

from the following claim summary info in TOON:

has_Medical_Events[29]:
  - Date_From: 2025-8-14
    Date_To: 2025-8-14
    Diagnosis: (B182) Chronic viral hepatitis C
    Medical_Condition: "Liver disease, mild"
    Place_Of_Service: Independent Laboratory
    Procedures: (CPT4|87521) HEPATITIS C PROBE&RVRS TRNSC (Lab - Other)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-8-18
    Date_To: 2025-8-18
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam),(CPT4|99213) OFFICE O/P EST LOW 20 MIN (Office Visit - Established Patient)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-2-12
    Date_To: 2025-2-12
    Diagnosis: (I330) Acute and subacute infective endocarditis
    Medical_Condition: Valvular disease
    Place_Of_Service: Home
    Procedures[2]: (CPT4|96365) THER/PROPH/DIAG IV INF INIT (Minor Procedures - Other),(HCPCS|J0696) Ceftriaxone sodium injection (Injectable Drugs)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-1-17
    Date_To: 2025-1-17
    Diagnosis: (I330) Acute and subacute infective endocarditis
    Medical_Condition: Valvular disease
    Place_Of_Service: Inpatient Hospital
    Procedures: (CPT4|71250) CT THORAX DX C- (CT Scans)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-11-13
    Date_To: 2025-11-13
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|99213) OFFICE O/P EST LOW 20 MIN (Office Visit - Established Patient),(CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-9-15
    Date_To: 2025-9-15
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam),(CPT4|99213) OFFICE O/P EST LOW 20 MIN (Office Visit - Established Patient)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-1-15
    Date_To: 2025-1-22
    Diagnosis[3]: (I330) Acute and subacute infective endocarditis,"(F1120) Opioid dependence, uncomplicated","(R319) Hematuria, unspecified"
    Medical_Condition[2]: Valvular disease,Drug abuse
    Place_Of_Service: Inpatient Hospital
    Procedures[2]: (CPT4|96365) THER/PROPH/DIAG IV INF INIT (Minor Procedures - Other),(HCPCS|J0696) Ceftriaxone sodium injection (Injectable Drugs)
    Span_Days: 8
    Type: medical_claim
  - Date_From: 2025-2-5
    Date_To: 2025-2-5
    Diagnosis: (I330) Acute and subacute infective endocarditis
    Medical_Condition: Valvular disease
    Place_Of_Service: Home
    Procedures[2]: (HCPCS|J0696) Ceftriaxone sodium injection (Injectable Drugs),(CPT4|96365) THER/PROPH/DIAG IV INF INIT (Minor Procedures - Other)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-3-5
    Date_To: 2025-3-5
    Diagnosis[2]: (B182) Chronic viral hepatitis C,"(F1120) Opioid dependence, uncomplicated"
    Medical_Condition[2]: Drug abuse,"Liver disease, mild"
    Place_Of_Service: Office
    Procedures[3]: (CPT4|87521) HEPATITIS C PROBE&RVRS TRNSC (Lab - Other),(CPT4|99204) OFFICE O/P NEW MOD 45 MIN (Office Visit - New Patient),(CPT4|80076) HEPATIC FUNCTION PANEL (Lab - Blood Tests)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-1-21
    Date_To: 2025-1-21
    Diagnosis: (Z23) Encounter for immunization
    Place_Of_Service: Inpatient Hospital
    Procedures: (CPT4|90732) PPSV23 VACC 2 YRS+ SUBQ/IM (Immunizations)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-7-17
    Date_To: 2025-7-17
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|99213) OFFICE O/P EST LOW 20 MIN (Office Visit - Established Patient),(CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-5-15
    Date_To: 2025-5-15
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam),(CPT4|99213) OFFICE O/P EST LOW 20 MIN (Office Visit - Established Patient)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-3-20
    Date_To: 2025-3-20
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam),(CPT4|99213) OFFICE O/P EST LOW 20 MIN (Office Visit - Established Patient)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-2-20
    Date_To: 2025-2-20
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|99204) OFFICE O/P NEW MOD 45 MIN (Office Visit - New Patient),(CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-1-29
    Date_To: 2025-1-29
    Diagnosis: (I330) Acute and subacute infective endocarditis
    Medical_Condition: Valvular disease
    Place_Of_Service: Home
    Procedures[2]: (CPT4|96365) THER/PROPH/DIAG IV INF INIT (Minor Procedures - Other),(HCPCS|J0696) Ceftriaxone sodium injection (Injectable Drugs)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-10-16
    Date_To: 2025-10-16
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam),(CPT4|99213) OFFICE O/P EST LOW 20 MIN (Office Visit - Established Patient)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-1-16
    Date_To: 2025-1-16
    Diagnosis[2]: (Z1159) Encounter for screening for other viral diseases,"(Z114) Encounter for screening for human immunodeficiency virus [HIV]"
    Place_Of_Service: Independent Laboratory
    Procedures[3]: (CPT4|86703) HIV-1/HIV-2 1 RESULT ANTBDY (Lab - Other),(CPT4|87340) HEPATITIS B SURFACE AG IA (Lab - Other),(CPT4|86803) HEPATITIS C AB TEST (Lab - Other)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-4-17
    Date_To: 2025-4-17
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam),(CPT4|99213) OFFICE O/P EST LOW 20 MIN (Office Visit - Established Patient)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-2-16
    Date_To: 2025-2-16
    Diagnosis[2]: (Z09) Encounter for follow-up examination after completed treatment for conditions other than malignant neoplasm,(I330) Acute and subacute infective endocarditis
    Medical_Condition: Valvular disease
    Place_Of_Service: Office
    Procedures: (CPT4|99213) OFFICE O/P EST LOW 20 MIN (Office Visit - Established Patient)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-6-19
    Date_To: 2025-6-19
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam),(CPT4|99214) OFFICE O/P EST MOD 30 MIN (Office Visit - Established Patient)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-12-15
    Date_To: 2025-12-15
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[2]: (CPT4|99213) OFFICE O/P EST LOW 20 MIN (Office Visit - Established Patient),(CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-1-16
    Date_To: 2025-1-16
    Diagnosis: (Z111) Encounter for screening for respiratory tuberculosis
    Place_Of_Service: Independent Laboratory
    Procedures: (CPT4|86480) TB TEST CELL IMMUN MEASURE (Lab - Blood Tests)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-6-10
    Date_To: 2025-6-10
    Diagnosis[2]: "(L0390) Cellulitis, unspecified","(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Emergency Room - Hospital
    Procedures[2]: (CPT4|99284) EMERGENCY DEPT VISIT MOD MDM (Emergency Room),(CPT4|10060) I&D ABSCESS SIMPLE/SINGLE (Other Dermatology)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-1-15
    Date_To: 2025-1-15
    Place_Of_Service: Emergency Room - Hospital
    Procedures: (CPT4|71046) X-RAY EXAM CHEST 2 VIEWS (Imaging/Radiology - Other)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-7-15
    Date_To: 2025-7-15
    Diagnosis: (Z8619) Personal history of other infectious and parasitic diseases
    Place_Of_Service: Office
    Procedures: (CPT4|93306) TTE W/DOPPLER COMPLETE (Cardiac Ultrasound/Doppler)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-1-15
    Date_To: 2025-1-15
    Diagnosis[2]: "(F1120) Opioid dependence, uncomplicated","(R319) Hematuria, unspecified"
    Medical_Condition: Drug abuse
    Place_Of_Service: Emergency Room - Hospital
    Procedures[3]: (CPT4|93000) ELECTROCARDIOGRAM COMPLETE (Cardiography - Includes Stress Testing),(CPT4|99285) EMERGENCY DEPT VISIT HI MDM (Emergency Room),(CPT4|87040) BLOOD CULTURE FOR BACTERIA (Lab - Other)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-1-16
    Date_To: 2025-1-16
    Diagnosis[2]: (I330) Acute and subacute infective endocarditis,"(F1120) Opioid dependence, uncomplicated"
    Medical_Condition[2]: Valvular disease,Drug abuse
    Place_Of_Service: Inpatient Hospital
    Procedures: (CPT4|93306) TTE W/DOPPLER COMPLETE (Cardiac Ultrasound/Doppler)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2026-1-14
    Date_To: 2026-1-14
    Diagnosis: "(F1120) Opioid dependence, uncomplicated"
    Medical_Condition: Drug abuse
    Place_Of_Service: Office
    Procedures[3]: (CPT4|80307) DRUG TEST PRSMV CHEM ANLYZR (Lab - Body Fluid Exam),(CPT4|80053) COMPREHEN METABOLIC PANEL (Lab - Blood Tests),(CPT4|99214) OFFICE O/P EST MOD 30 MIN (Office Visit - Established Patient)
    Span_Days: 1
    Type: medical_claim
  - Date_From: 2025-1-20
    Date_To: 2025-1-20
    Diagnosis: (Z23) Encounter for immunization
    Place_Of_Service: Inpatient Hospital
    Procedures[3]: (CPT4|90714) TD VACC NO PRESV 7 YRS+ IM (Immunizations),(CPT4|90632) HEPA VACCINE ADULT IM (Immunizations),(CPT4|90746) HEPB VACCINE 3 DOSE ADULT IM (Immunizations)
    Span_Days: 1
    Type: medical_claim
has_Pharmacy_Events[5]:
  - Adherence_Ratio: 0.9090909090909091
    Brand_Generic_Code: G
    Date_From: 2025-6-10
    Date_To: 2025-6-10
    Drug_Class_Name: *Opioid Agonists**
    Drug_Name: traMADol HCl
    Drug_Strength: "50"
    Drug_Type_Code: Rx
    GPI: "65100095100320"
    Generic_Name: Tramadol HCl Tablet
    Maintenance_Drug_Indicator: N
    NDC: "00093005801"
    Span_Days: 11
    Total_Days_Supply: 10
    Total_Quantity_Dispensed: 20
    Type: pharmacy_drug
    Unit_Of_Measure: MG
  - Adherence_Ratio: 0.2857142857142857
    Brand_Generic_Code: G
    Date_From: 2025-2-20
    Date_To: 2025-8-18
    Drug_Class_Name: *Opioid Antagonists**
    Drug_Name: Narcan
    Drug_Strength: "4"
    Drug_Type_Code: Rx
    GPI: "93400020100920"
    Generic_Name: Naloxone HCl Liquid
    Maintenance_Drug_Indicator: N
    NDC: "69547035302"
    Span_Days: 210
    Total_Days_Supply: 60
    Total_Quantity_Dispensed: 4
    Type: pharmacy_drug
    Unit_Of_Measure: MG/0.1ML
  - Adherence_Ratio: 0.9019607843137255
    Date_From: 2025-2-20
    Date_To: 2026-1-14
    NDC: "42858011203"
    Span_Days: 357
    Total_Days_Supply: 322
    Total_Quantity_Dispensed: 644
    Type: pharmacy_drug
  - Adherence_Ratio: 0.9333333333333333
    Brand_Generic_Code: B
    Date_From: 2025-3-5
    Date_To: 2025-5-5
    Drug_Class_Name: *Hepatitis Agents**
    Drug_Name: Epclusa
    Drug_Strength: 400-100
    Drug_Type_Code: Rx
    GPI: "12359902650330"
    Generic_Name: Sofosbuvir-Velpatasvir Tablet
    Maintenance_Drug_Indicator: N
    NDC: "61958220101"
    Span_Days: 90
    Total_Days_Supply: 84
    Total_Quantity_Dispensed: 84
    Type: pharmacy_drug
    Unit_Of_Measure: MG
  - Adherence_Ratio: 0.9655172413793104
    Date_From: 2025-1-22
    Date_To: 2025-2-12
    NDC: "00409754511"
    Span_Days: 29
    Total_Days_Supply: 28
    Total_Quantity_Dispensed: 28
    Type: pharmacy_drug
