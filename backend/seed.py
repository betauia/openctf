import json
from app.db import SessionLocal, Base, engine
from app.db.models.challenge import Challenge

Base.metadata.create_all(bind=engine)

def f(*flags):  return json.dumps(list(flags))
def fs(*files): return json.dumps(list(files))

challenges = [
    Challenge(title="SQL Safari",       category="web",      points=100, difficulty="Easy",   flags=f("CTF{sql_safari_flag}"),       author="arch", solves=0, is_visible=True, 
              description="Basic SQL injection challenge."),
    Challenge(title="XSS Labyrinth",    category="web",      points=250, difficulty="Medium", flags=f("CTF{xss_labyrinth_flag}"),    author="arch", solves=0, is_visible=True, 
              description="Stored XSS in a comment field."),
    Challenge(title="SSRF Descent",     category="web",      points=400, difficulty="Hard",   flags=f("CTF{ssrf_descent_flag}"),     author="arch", solves=0, is_visible=True, 
              description="Bypass SSRF protections to reach internal services."),
    Challenge(title="ret2libc Classic", category="pwn",      points=150, difficulty="Easy",   flags=f("CTF{ret2libc_classic_flag}"), author="arch", solves=0, is_visible=True, 
              description="Classic return-to-libc exploit.",                       
              connection_info="nc pwn.openctf.local 1337"),
    Challenge(title="Heap Havoc",       category="pwn",      points=350, difficulty="Hard",   flags=f("CTF{heap_havoc_flag}"),       author="arch", solves=0, is_visible=True, 
              description="Exploit a use-after-free vulnerability.",              
              connection_info="nc pwn.openctf.local 1338"),
    Challenge(title="RSA Baby Steps",   category="crypto",   points=75,  difficulty="Easy",   flags=f("CTF{rsa_baby_steps_flag}"),   author="arch", solves=0, is_visible=True, 
              description="Small exponent RSA attack."),
    Challenge(title="AES-CBC Oracle",   category="crypto",   points=300, difficulty="Medium", flags=f("CTF{aes_cbc_oracle_flag}"),   author="arch", solves=0, is_visible=True, 
              description="Padding oracle attack on AES-CBC."),
    Challenge(title="Crackme 001",      category="rev",      points=100, difficulty="Easy",   flags=f("CTF{crackme_001_flag}"),      author="arch", solves=0, is_visible=True, 
              description="Simple x86 crackme."),
    Challenge(title="VM Escape",        category="rev",      points=450, difficulty="Hard",   flags=f("CTF{vm_escape_flag}"),        author="arch", solves=0, is_visible=True, 
              description="Reverse a custom bytecode VM and escape it."),
    Challenge(title="JTAG Whisper",     category="hardware", points=200, difficulty="Medium", flags=f("CTF{jtag_whisper_flag}"),     author="arch", solves=0, is_visible=True, 
              description="Extract firmware over JTAG."),
    Challenge(title="Steg & Destroy",   category="misc",     points=125, difficulty="Easy",   flags=f("CTF{steg_destroy_flag}"),     author="arch", solves=0, is_visible=True, 
              description="Find the hidden message in the image."),
    Challenge(title="Forensics Fiend",  category="misc",     points=275, difficulty="Medium", flags=f("CTF{forensics_fiend_flag}"),  author="arch", solves=0, is_visible=True, 
              description="Memory dump analysis."),
    Challenge(title="Stack Siege",      category="pwn",      points=200, difficulty="Medium", flags=f("CTF{stack_siege_flag}"),      author="arch", solves=0, is_visible=True,
              description="Exploit a classic stack buffer overflow. Download the binary and connect to the remote service.",
              connection_info="nc pwn.openctf.local 1339", files=fs("stack_siege")),
]

db = SessionLocal()
db.add_all(challenges)
db.commit()
db.close()
print(f"Seeded {len(challenges)} challenges.")
