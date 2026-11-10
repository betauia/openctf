"""Prints SQL that seeds the challenges table. Titles that already exist are skipped.

    python3 backend/seed.py | docker compose exec -T database psql -U openctf -d openctf
"""
import sys

AUTHOR = "arch"
COLUMNS = ["title", "category", "points", "difficulty", "description", "connection_info", "files", "flags"]

CHALLENGES = [
    ("SQL Safari",       "web",       100, "Easy",   "Basic SQL injection challenge.",                                                                    None,                        [],               ["CTF{sql_safari_flag}"]),
    ("XSS Labyrinth",    "web",       250, "Medium", "Stored XSS in a comment field.",                                                                    None,                        [],               ["CTF{xss_labyrinth_flag}"]),
    ("SSRF Descent",     "web",       400, "Hard",   "Bypass SSRF protections to reach internal services.",                                               None,                        [],               ["CTF{ssrf_descent_flag}"]),
    ("ret2libc Classic", "pwn",       150, "Easy",   "Classic return-to-libc exploit.",                                                                   "nc pwn.openctf.local 1337", [],               ["CTF{ret2libc_classic_flag}"]),
    ("Heap Havoc",       "pwn",       350, "Hard",   "Exploit a use-after-free vulnerability.",                                                           "nc pwn.openctf.local 1338", [],               ["CTF{heap_havoc_flag}"]),
    ("Stack Siege",      "pwn",       200, "Medium", "Exploit a classic stack buffer overflow. Download the binary and connect to the remote service.",   "nc pwn.openctf.local 1339", ["stack_siege"],  ["CTF{stack_siege_flag}"]),
    ("RSA Baby Steps",   "crypto",     75, "Easy",   "Small exponent RSA attack.",                                                                        None,                        [],               ["CTF{rsa_baby_steps_flag}"]),
    ("AES-CBC Oracle",   "crypto",    300, "Medium", "Padding oracle attack on AES-CBC.",                                                                 None,                        [],               ["CTF{aes_cbc_oracle_flag}"]),
    ("Crackme 001",      "rev",       100, "Easy",   "Simple x86 crackme.",                                                                               None,                        [],               ["CTF{crackme_001_flag}"]),
    ("VM Escape",        "rev",       450, "Hard",   "Reverse a custom bytecode VM and escape it.",                                                       None,                        [],               ["CTF{vm_escape_flag}"]),
    ("Unpack Me",        "rev",       200, "Medium", "A packed binary conceals its secrets. Unpack and reverse it.",                                      None,                        [],               ["CTF{unpack_me_flag}"]),
    ("JTAG Whisper",     "hardware",  200, "Medium", "Extract firmware over JTAG.",                                                                       None,                        [],               ["CTF{jtag_whisper_flag}"]),
    ("Steg & Destroy",   "misc",      125, "Easy",   "Find the hidden message in the image.",                                                             None,                        [],               ["CTF{steg_destroy_flag}"]),
    ("Forensics Fiend",  "misc",      275, "Medium", "Memory dump analysis.",                                                                             None,                        [],               ["CTF{forensics_fiend_flag}"]),
    ("Hello World",      "beginner",   10, "Easy",   "Just read the flag. Welcome to the CTF!",                                                           None,                        [],               ["CTF{welcome_to_the_ctf}"]),
    ("Idle No More",     "afk",        50, "Easy",   "Submit the flag. Yes, really.",                                                                     None,                        [],               ["CTF{you_came_back}"]),
    ("Fix My Script",    "coding",    150, "Easy",   "A broken Python script hides the flag. Fix it and run it.",                                         None,                        [],               ["CTF{fix_my_script_flag}"]),
    ("Log Digger",       "forensics", 175, "Easy",   "Dig through the access logs and find the suspicious request.",                                      None,                        [],               ["CTF{log_digger_flag}"]),
    ("Insta Stalker",    "osint",     125, "Easy",   "Find the flag hidden in a public social media profile.",                                            None,                        [],               ["CTF{insta_stalker_flag}"]),
    ("Pixel Secrets",    "stego",     100, "Easy",   "Something is hidden in this image. Can you find it?",                                               None,                        [],               ["CTF{pixel_secrets_flag}"]),
]


def quote(value):
    if value is None:
        return "NULL"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, list):
        return "ARRAY[" + ", ".join(quote(v) for v in value) + "]::text[]"
    return "'" + value.replace("'", "''") + "'"


def row(challenge):
    *columns, flags = challenge
    return "(" + ", ".join(quote(c) for c in columns) + f", to_jsonb({quote(flags)}))"


columns = ", ".join(COLUMNS)
rows = ",\n    ".join(row(c) for c in CHALLENGES)
STATEMENT = f"""INSERT INTO challenges ({columns}, author, is_visible)
SELECT v.*, {quote(AUTHOR)}, TRUE FROM (VALUES
    {rows}
) AS v ({columns})
WHERE NOT EXISTS (SELECT 1 FROM challenges c WHERE c.title = v.title);
"""

if __name__ == "__main__":
    if "--check" in sys.argv:
        assert quote("it's") == "'it''s'"
        assert quote(["a"]) == "ARRAY['a']::text[]"
        assert STATEMENT.count("to_jsonb(") == len(CHALLENGES)
        print("ok")
    else:
        print(STATEMENT)
