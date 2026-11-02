"""Authoritative Sri Lankan City, Town and Suburb to 5-digit Postal Code directory.

Covers major cities, commercial towns, postal divisions, and regional sub-post offices
across all 25 administrative districts in Sri Lanka.
"""

from typing import Any, Dict, List, Optional

SRI_LANKA_POSTAL_DIRECTORY: List[Dict[str, str]] = [
    # --------------------------------------------------------------------------
    # Western Province - Colombo District
    # --------------------------------------------------------------------------
    {"city": "Colombo 01 (Fort)", "alias": "fort", "code": "00100", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 02 (Slave Island)", "alias": "slave island", "code": "00200", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 03 (Kollupitiya)", "alias": "kollupitiya", "code": "00300", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 04 (Bambalapitiya)", "alias": "bambalapitiya", "code": "00400", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 05 (Havelock / Kirulapone)", "alias": "havelock", "code": "00500", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 06 (Wellawatte)", "alias": "wellawatte", "code": "00600", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 07 (Cinnamon Gardens)", "alias": "cinnamon gardens", "code": "00700", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 08 (Borella)", "alias": "borella", "code": "00800", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 09 (Dematagoda)", "alias": "dematagoda", "code": "00900", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 10 (Maradana / Panchikawatte)", "alias": "maradana", "code": "01000", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 11 (Pettah)", "alias": "pettah", "code": "01100", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 12 (Hultsdorf)", "alias": "hultsdorf", "code": "01200", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 13 (Kotahena / Bloemendhal)", "alias": "kotahena", "code": "01300", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 14 (Grandpass)", "alias": "grandpass", "code": "01400", "district": "Colombo", "province": "Western"},
    {"city": "Colombo 15 (Mutwal / Modara)", "alias": "mutwal", "code": "01500", "district": "Colombo", "province": "Western"},
    {"city": "Colombo", "alias": "colombo", "code": "00100", "district": "Colombo", "province": "Western"},
    {"city": "Kotte", "alias": "sri jayawardenepura", "code": "10100", "district": "Colombo", "province": "Western"},
    {"city": "Rajagiriya", "alias": "rajagiriya", "code": "10107", "district": "Colombo", "province": "Western"},
    {"city": "Battaramulla", "alias": "battaramulla", "code": "10120", "district": "Colombo", "province": "Western"},
    {"city": "Malabe", "alias": "malabe", "code": "10115", "district": "Colombo", "province": "Western"},
    {"city": "Pelawatte", "alias": "pelawatte", "code": "10120", "district": "Colombo", "province": "Western"},
    {"city": "Kaduwela", "alias": "kaduwela", "code": "10640", "district": "Colombo", "province": "Western"},
    {"city": "Athurugiriya", "alias": "athurugiriya", "code": "10150", "district": "Colombo", "province": "Western"},
    {"city": "Maharagama", "alias": "maharagama", "code": "10280", "district": "Colombo", "province": "Western"},
    {"city": "Nugegoda", "alias": "nugegoda", "code": "10250", "district": "Colombo", "province": "Western"},
    {"city": "Kohuwala", "alias": "kohuwala", "code": "10250", "district": "Colombo", "province": "Western"},
    {"city": "Dehiwala", "alias": "dehiwala", "code": "10350", "district": "Colombo", "province": "Western"},
    {"city": "Mount Lavinia", "alias": "mount lavinia", "code": "10370", "district": "Colombo", "province": "Western"},
    {"city": "Ratmalana", "alias": "ratmalana", "code": "10390", "district": "Colombo", "province": "Western"},
    {"city": "Moratuwa", "alias": "moratuwa", "code": "10400", "district": "Colombo", "province": "Western"},
    {"city": "Pannipitiya", "alias": "pannipitiya", "code": "10230", "district": "Colombo", "province": "Western"},
    {"city": "Kottawa", "alias": "kottawa", "code": "10230", "district": "Colombo", "province": "Western"},
    {"city": "Homagama", "alias": "homagama", "code": "10206", "district": "Colombo", "province": "Western"},
    {"city": "Padukka", "alias": "padukka", "code": "10500", "district": "Colombo", "province": "Western"},
    {"city": "Hanwella", "alias": "hanwella", "code": "10650", "district": "Colombo", "province": "Western"},
    {"city": "Avissawella", "alias": "avissawella", "code": "10700", "district": "Colombo", "province": "Western"},

    # --------------------------------------------------------------------------
    # Western Province - Gampaha District
    # --------------------------------------------------------------------------
    {"city": "Gampaha", "alias": "gampaha", "code": "11000", "district": "Gampaha", "province": "Western"},
    {"city": "Negombo", "alias": "negombo", "code": "11500", "district": "Gampaha", "province": "Western"},
    {"city": "Kelaniya", "alias": "kelaniya", "code": "11600", "district": "Gampaha", "province": "Western"},
    {"city": "Kiribathgoda", "alias": "kiribathgoda", "code": "11600", "district": "Gampaha", "province": "Western"},
    {"city": "Kadawatha", "alias": "kadawatha", "code": "11850", "district": "Gampaha", "province": "Western"},
    {"city": "Wattala", "alias": "wattala", "code": "11300", "district": "Gampaha", "province": "Western"},
    {"city": "Ja-Ela", "alias": "ja-ela", "code": "11350", "district": "Gampaha", "province": "Western"},
    {"city": "Kandana", "alias": "kandana", "code": "11320", "district": "Gampaha", "province": "Western"},
    {"city": "Ragama", "alias": "ragama", "code": "11010", "district": "Gampaha", "province": "Western"},
    {"city": "Seeduwa", "alias": "seeduwa", "code": "11410", "district": "Gampaha", "province": "Western"},
    {"city": "Katunayake", "alias": "katunayake", "code": "11450", "district": "Gampaha", "province": "Western"},
    {"city": "Minuwangoda", "alias": "minuwangoda", "code": "11550", "district": "Gampaha", "province": "Western"},
    {"city": "Divulapitiya", "alias": "divulapitiya", "code": "11200", "district": "Gampaha", "province": "Western"},
    {"city": "Mirigama", "alias": "mirigama", "code": "11200", "district": "Gampaha", "province": "Western"},
    {"city": "Veyangoda", "alias": "veyangoda", "code": "11100", "district": "Gampaha", "province": "Western"},
    {"city": "Nittambuwa", "alias": "nittambuwa", "code": "11880", "district": "Gampaha", "province": "Western"},
    {"city": "Yakkala", "alias": "yakkala", "code": "11870", "district": "Gampaha", "province": "Western"},

    # --------------------------------------------------------------------------
    # Western Province - Kalutara District
    # --------------------------------------------------------------------------
    {"city": "Kalutara", "alias": "kalutara", "code": "12000", "district": "Kalutara", "province": "Western"},
    {"city": "Panadura", "alias": "panadura", "code": "12500", "district": "Kalutara", "province": "Western"},
    {"city": "Wadduwa", "alias": "wadduwa", "code": "12560", "district": "Kalutara", "province": "Western"},
    {"city": "Beruwala", "alias": "beruwala", "code": "12070", "district": "Kalutara", "province": "Western"},
    {"city": "Aluthgama", "alias": "aluthgama", "code": "12080", "district": "Kalutara", "province": "Western"},
    {"city": "Horana", "alias": "horana", "code": "12400", "district": "Kalutara", "province": "Western"},
    {"city": "Bandaragama", "alias": "bandaragama", "code": "12530", "district": "Kalutara", "province": "Western"},
    {"city": "Matugama", "alias": "matugama", "code": "12100", "district": "Kalutara", "province": "Western"},

    # --------------------------------------------------------------------------
    # Central Province - Kandy District (Includes Kadugannawa)
    # --------------------------------------------------------------------------
    {"city": "Kandy", "alias": "kandy", "code": "20000", "district": "Kandy", "province": "Central"},
    {"city": "Kadugannawa", "alias": "kadugannawa", "code": "20430", "district": "Kandy", "province": "Central"},
    {"city": "Peradeniya", "alias": "peradeniya", "code": "20400", "district": "Kandy", "province": "Central"},
    {"city": "Pilimathalawa", "alias": "pilimathalawa", "code": "20450", "district": "Kandy", "province": "Central"},
    {"city": "Katugastota", "alias": "katugastota", "code": "20120", "district": "Kandy", "province": "Central"},
    {"city": "Gampola", "alias": "gampola", "code": "20500", "district": "Kandy", "province": "Central"},
    {"city": "Nawalapitiya", "alias": "nawalapitiya", "code": "20650", "district": "Kandy", "province": "Central"},
    {"city": "Kundasale", "alias": "kundasale", "code": "20168", "district": "Kandy", "province": "Central"},
    {"city": "Digana", "alias": "digana", "code": "20180", "district": "Kandy", "province": "Central"},
    {"city": "Wattegama", "alias": "wattegama", "code": "20810", "district": "Kandy", "province": "Central"},
    {"city": "Akurana", "alias": "akurana", "code": "20850", "district": "Kandy", "province": "Central"},
    {"city": "Gelioya", "alias": "gelioya", "code": "20440", "district": "Kandy", "province": "Central"},
    {"city": "Teldeniya", "alias": "teldeniya", "code": "20187", "district": "Kandy", "province": "Central"},

    # --------------------------------------------------------------------------
    # Central Province - Matale & Nuwara Eliya
    # --------------------------------------------------------------------------
    {"city": "Matale", "alias": "matale", "code": "21000", "district": "Matale", "province": "Central"},
    {"city": "Dambulla", "alias": "dambulla", "code": "21100", "district": "Matale", "province": "Central"},
    {"city": "Galewela", "alias": "galewela", "code": "21200", "district": "Matale", "province": "Central"},
    {"city": "Nuwara Eliya", "alias": "nuwara eliya", "code": "22200", "district": "Nuwara Eliya", "province": "Central"},
    {"city": "Hatton", "alias": "hatton", "code": "22000", "district": "Nuwara Eliya", "province": "Central"},
    {"city": "Talawakele", "alias": "talawakele", "code": "22100", "district": "Nuwara Eliya", "province": "Central"},

    # --------------------------------------------------------------------------
    # Southern Province - Hambantota District (Includes Walasmulla)
    # --------------------------------------------------------------------------
    {"city": "Hambantota", "alias": "hambantota", "code": "82000", "district": "Hambantota", "province": "Southern"},
    {"city": "Walasmulla", "alias": "walasmulla", "code": "82450", "district": "Hambantota", "province": "Southern"},
    {"city": "Tangalle", "alias": "tangalle", "code": "82200", "district": "Hambantota", "province": "Southern"},
    {"city": "Beliatta", "alias": "beliatta", "code": "82400", "district": "Hambantota", "province": "Southern"},
    {"city": "Ambalantota", "alias": "ambalantota", "code": "82100", "district": "Hambantota", "province": "Southern"},
    {"city": "Tissamaharama", "alias": "tissamaharama", "code": "82600", "district": "Hambantota", "province": "Southern"},
    {"city": "Weeraketiya", "alias": "weeraketiya", "code": "82240", "district": "Hambantota", "province": "Southern"},
    {"city": "Ranna", "alias": "ranna", "code": "82240", "district": "Hambantota", "province": "Southern"},
    {"city": "Middeniya", "alias": "middeniya", "code": "82270", "district": "Hambantota", "province": "Southern"},
    {"city": "Katuwana", "alias": "katuwana", "code": "82500", "district": "Hambantota", "province": "Southern"},

    # --------------------------------------------------------------------------
    # Southern Province - Galle & Matara Districts
    # --------------------------------------------------------------------------
    {"city": "Galle", "alias": "galle", "code": "80000", "district": "Galle", "province": "Southern"},
    {"city": "Karapitiya", "alias": "karapitiya", "code": "80000", "district": "Galle", "province": "Southern"},
    {"city": "Hikkaduwa", "alias": "hikkaduwa", "code": "80240", "district": "Galle", "province": "Southern"},
    {"city": "Ambalangoda", "alias": "ambalangoda", "code": "80300", "district": "Galle", "province": "Southern"},
    {"city": "Elpitiya", "alias": "elpitiya", "code": "80400", "district": "Galle", "province": "Southern"},
    {"city": "Baddegama", "alias": "baddegama", "code": "80200", "district": "Galle", "province": "Southern"},
    {"city": "Bentota", "alias": "bentota", "code": "80500", "district": "Galle", "province": "Southern"},
    {"city": "Matara", "alias": "matara", "code": "81000", "district": "Matara", "province": "Southern"},
    {"city": "Weligama", "alias": "weligama", "code": "81700", "district": "Matara", "province": "Southern"},
    {"city": "Dikwella", "alias": "dikwella", "code": "81100", "district": "Matara", "province": "Southern"},
    {"city": "Akuressa", "alias": "akuressa", "code": "81300", "district": "Matara", "province": "Southern"},
    {"city": "Kamburupitiya", "alias": "kamburupitiya", "code": "81100", "district": "Matara", "province": "Southern"},
    {"city": "Hakmana", "alias": "hakmana", "code": "81370", "district": "Matara", "province": "Southern"},
    {"city": "Deniyaya", "alias": "deniyaya", "code": "81500", "district": "Matara", "province": "Southern"},

    # --------------------------------------------------------------------------
    # Sabaragamuwa Province - Kegalle & Ratnapura Districts
    # --------------------------------------------------------------------------
    {"city": "Kegalle", "alias": "kegalle", "code": "71000", "district": "Kegalle", "province": "Sabaragamuwa"},
    {"city": "Mawanella", "alias": "mawanella", "code": "71500", "district": "Kegalle", "province": "Sabaragamuwa"},
    {"city": "Warakapola", "alias": "warakapola", "code": "71600", "district": "Kegalle", "province": "Sabaragamuwa"},
    {"city": "Rambukkana", "alias": "rambukkana", "code": "71100", "district": "Kegalle", "province": "Sabaragamuwa"},
    {"city": "Ruwanwella", "alias": "ruwanwella", "code": "71350", "district": "Kegalle", "province": "Sabaragamuwa"},
    {"city": "Yatiyantota", "alias": "yatiyantota", "code": "71720", "district": "Kegalle", "province": "Sabaragamuwa"},
    {"city": "Dehiowita", "alias": "dehiowita", "code": "71400", "district": "Kegalle", "province": "Sabaragamuwa"},
    {"city": "Ratnapura", "alias": "ratnapura", "code": "70000", "district": "Ratnapura", "province": "Sabaragamuwa"},
    {"city": "Balangoda", "alias": "balangoda", "code": "70100", "district": "Ratnapura", "province": "Sabaragamuwa"},
    {"city": "Pelmadulla", "alias": "pelmadulla", "code": "70070", "district": "Ratnapura", "province": "Sabaragamuwa"},
    {"city": "Embilipitiya", "alias": "embilipitiya", "code": "70200", "district": "Ratnapura", "province": "Sabaragamuwa"},
    {"city": "Kuruwita", "alias": "kuruwita", "code": "70500", "district": "Ratnapura", "province": "Sabaragamuwa"},
    {"city": "Eheliyagoda", "alias": "eheliyagoda", "code": "70600", "district": "Ratnapura", "province": "Sabaragamuwa"},

    # --------------------------------------------------------------------------
    # North Western Province - Kurunegala & Puttalam Districts
    # --------------------------------------------------------------------------
    {"city": "Kurunegala", "alias": "kurunegala", "code": "60000", "district": "Kurunegala", "province": "North Western"},
    {"city": "Kuliyapitiya", "alias": "kuliyapitiya", "code": "60200", "district": "Kurunegala", "province": "North Western"},
    {"city": "Pannala", "alias": "pannala", "code": "60160", "district": "Kurunegala", "province": "North Western"},
    {"city": "Narammala", "alias": "narammala", "code": "60100", "district": "Kurunegala", "province": "North Western"},
    {"city": "Alawwa", "alias": "alawwa", "code": "60280", "district": "Kurunegala", "province": "North Western"},
    {"city": "Giriulla", "alias": "giriulla", "code": "60140", "district": "Kurunegala", "province": "North Western"},
    {"city": "Polgahawela", "alias": "polgahawela", "code": "60300", "district": "Kurunegala", "province": "North Western"},
    {"city": "Wariyapola", "alias": "wariyapola", "code": "60400", "district": "Kurunegala", "province": "North Western"},
    {"city": "Nikaweratiya", "alias": "nikaweratiya", "code": "60470", "district": "Kurunegala", "province": "North Western"},
    {"city": "Puttalam", "alias": "puttalam", "code": "61300", "district": "Puttalam", "province": "North Western"},
    {"city": "Chilaw", "alias": "chilaw", "code": "61000", "district": "Puttalam", "province": "North Western"},
    {"city": "Wennappuwa", "alias": "wennappuwa", "code": "61170", "district": "Puttalam", "province": "North Western"},
    {"city": "Marawila", "alias": "marawila", "code": "61150", "district": "Puttalam", "province": "North Western"},
    {"city": "Dankotuwa", "alias": "dankotuwa", "code": "61180", "district": "Puttalam", "province": "North Western"},

    # --------------------------------------------------------------------------
    # North Central Province - Anuradhapura & Polonnaruwa Districts
    # --------------------------------------------------------------------------
    {"city": "Anuradhapura", "alias": "anuradhapura", "code": "50000", "district": "Anuradhapura", "province": "North Central"},
    {"city": "Kekirawa", "alias": "kekirawa", "code": "50100", "district": "Anuradhapura", "province": "North Central"},
    {"city": "Medawachchiya", "alias": "medawachchiya", "code": "50500", "district": "Anuradhapura", "province": "North Central"},
    {"city": "Tambuttegama", "alias": "tambuttegama", "code": "50150", "district": "Anuradhapura", "province": "North Central"},
    {"city": "Eppawala", "alias": "eppawala", "code": "50180", "district": "Anuradhapura", "province": "North Central"},
    {"city": "Mihintale", "alias": "mihintale", "code": "50300", "district": "Anuradhapura", "province": "North Central"},
    {"city": "Habarana", "alias": "habarana", "code": "50146", "district": "Anuradhapura", "province": "North Central"},
    {"city": "Nochchiyagama", "alias": "nochchiyagama", "code": "50200", "district": "Anuradhapura", "province": "North Central"},
    {"city": "Polonnaruwa", "alias": "polonnaruwa", "code": "51000", "district": "Polonnaruwa", "province": "North Central"},
    {"city": "Kaduruwela", "alias": "kaduruwela", "code": "51000", "district": "Polonnaruwa", "province": "North Central"},
    {"city": "Hingurakgoda", "alias": "hingurakgoda", "code": "51400", "district": "Polonnaruwa", "province": "North Central"},
    {"city": "Medirigiriya", "alias": "medirigiriya", "code": "51500", "district": "Polonnaruwa", "province": "North Central"},

    # --------------------------------------------------------------------------
    # Northern Province
    # --------------------------------------------------------------------------
    {"city": "Jaffna", "alias": "jaffna", "code": "40000", "district": "Jaffna", "province": "Northern"},
    {"city": "Chavakachcheri", "alias": "chavakachcheri", "code": "40500", "district": "Jaffna", "province": "Northern"},
    {"city": "Point Pedro", "alias": "point pedro", "code": "40600", "district": "Jaffna", "province": "Northern"},
    {"city": "Kilinochchi", "alias": "kilinochchi", "code": "44000", "district": "Kilinochchi", "province": "Northern"},
    {"city": "Mannar", "alias": "mannar", "code": "41000", "district": "Mannar", "province": "Northern"},
    {"city": "Vavuniya", "alias": "vavuniya", "code": "43000", "district": "Vavuniya", "province": "Northern"},
    {"city": "Mullaitivu", "alias": "mullaitivu", "code": "42000", "district": "Mullaitivu", "province": "Northern"},

    # --------------------------------------------------------------------------
    # Eastern Province
    # --------------------------------------------------------------------------
    {"city": "Trincomalee", "alias": "trincomalee", "code": "31000", "district": "Trincomalee", "province": "Eastern"},
    {"city": "Kinniya", "alias": "kinniya", "code": "31100", "district": "Trincomalee", "province": "Eastern"},
    {"city": "Batticaloa", "alias": "batticaloa", "code": "30000", "district": "Batticaloa", "province": "Eastern"},
    {"city": "Kattankudy", "alias": "kattankudy", "code": "30130", "district": "Batticaloa", "province": "Eastern"},
    {"city": "Valaichchenai", "alias": "valaichchenai", "code": "30400", "district": "Batticaloa", "province": "Eastern"},
    {"city": "Ampara", "alias": "ampara", "code": "32000", "district": "Ampara", "province": "Eastern"},
    {"city": "Kalmunai", "alias": "kalmunai", "code": "32300", "district": "Ampara", "province": "Eastern"},
    {"city": "Akkaraipattu", "alias": "akkaraipattu", "code": "32400", "district": "Ampara", "province": "Eastern"},

    # --------------------------------------------------------------------------
    # Uva Province - Badulla & Monaragala Districts
    # --------------------------------------------------------------------------
    {"city": "Badulla", "alias": "badulla", "code": "90000", "district": "Badulla", "province": "Uva"},
    {"city": "Bandarawela", "alias": "bandarawela", "code": "90100", "district": "Badulla", "province": "Uva"},
    {"city": "Ella", "alias": "ella", "code": "90090", "district": "Badulla", "province": "Uva"},
    {"city": "Haputale", "alias": "haputale", "code": "90160", "district": "Badulla", "province": "Uva"},
    {"city": "Mahiyanganaya", "alias": "mahiyanganaya", "code": "90700", "district": "Badulla", "province": "Uva"},
    {"city": "Welimada", "alias": "welimada", "code": "90200", "district": "Badulla", "province": "Uva"},
    {"city": "Monaragala", "alias": "monaragala", "code": "91200", "district": "Monaragala", "province": "Uva"},
    {"city": "Wellawaya", "alias": "wellawaya", "code": "91270", "district": "Monaragala", "province": "Uva"},
    {"city": "Bibile", "alias": "bibile", "code": "91500", "district": "Monaragala", "province": "Uva"},
    {"city": "Kataragama", "alias": "kataragama", "code": "91400", "district": "Monaragala", "province": "Uva"},
]

# Quick key-value mapping for fast O(1) exact lookups
SRI_LANKA_CITY_POSTAL_CODES: Dict[str, str] = {
    entry["alias"]: entry["code"] for entry in SRI_LANKA_POSTAL_DIRECTORY
}
# Also index by lowercase full city name
for entry in SRI_LANKA_POSTAL_DIRECTORY:
    SRI_LANKA_CITY_POSTAL_CODES[entry["city"].lower()] = entry["code"]


def lookup_postal_record(query: str) -> Optional[Dict[str, str]]:
    """Search the authoritative Sri Lankan postal directory by town, city, suburb or code.

    Matches exact name, alias, or substring. Returns complete record with district and province.
    """
    if not query or not isinstance(query, str):
        return None

    cleaned = query.strip().lower()

    # 1. Exact numeric code match
    if cleaned.isdigit() and len(cleaned) == 5:
        for entry in SRI_LANKA_POSTAL_DIRECTORY:
            if entry["code"] == cleaned:
                return dict(entry)
        return {"city": f"Postal Zone {cleaned}", "code": cleaned, "district": "Sri Lanka", "province": "National"}

    # 2. Exact alias or city match
    if cleaned in SRI_LANKA_CITY_POSTAL_CODES:
        code = SRI_LANKA_CITY_POSTAL_CODES[cleaned]
        for entry in SRI_LANKA_POSTAL_DIRECTORY:
            if entry["code"] == code:
                return dict(entry)

    # 3. Substring matching (e.g. "walasmulla town" -> "walasmulla", "kadugannawa junction" -> "kadugannawa")
    for entry in SRI_LANKA_POSTAL_DIRECTORY:
        alias = entry["alias"]
        city_lower = entry["city"].lower()
        if alias in cleaned or cleaned in alias or city_lower in cleaned:
            return dict(entry)

    return None


def resolve_postal_code(input_val: str) -> Optional[str]:
    """Resolve a 5-digit postal code or town name (e.g. 'Walasmulla', 'Kadugannawa') to 5-digit code."""
    record = lookup_postal_record(input_val)
    if record:
        return record.get("code")
    return None
