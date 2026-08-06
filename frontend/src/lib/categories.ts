import { Globe, Brain, Lock, Cpu, CircuitBoard, Aperture, Clock, Star, Terminal, Fingerprint, Eye, RotateCcw, Image } from "@lucide/astro";

export const CAT_DEFS = [
  { slug: "web",       color: "#2979ff" },
  { slug: "pwn",       color: "#ef5350" },
  { slug: "crypto",    color: "#ab47bc" },
  { slug: "rev",       color: "#ffa726" },
  { slug: "hardware",  color: "#ff7043" },
  { slug: "misc",      color: "#66bb6a" },
  { slug: "afk",       color: "#78909c" },
  { slug: "beginner",  color: "#26c6da" },
  { slug: "coding",    color: "#42a5f5" },
  { slug: "forensics", color: "#26a69a" },
  { slug: "osint",     color: "#ec407a" },
  { slug: "reversing", color: "#ffb74d" },
  { slug: "stego",     color: "#ce93d8" },
] as const;

export const CAT_SLUGS = CAT_DEFS.map(c => c.slug);

const ICONS: Record<string, any> = {
  web: Globe, pwn: Brain, crypto: Lock, rev: Cpu, hardware: CircuitBoard, misc: Aperture,
  afk: Clock, beginner: Star, coding: Terminal, forensics: Fingerprint, osint: Eye, reversing: RotateCcw, stego: Image,
};

export const CATS = CAT_DEFS.map(c => ({ ...c, Icon: ICONS[c.slug] }));
export const CAT_MAP = Object.fromEntries(CATS.map(c => [c.slug, c]));

export const DIFF_DEFS = [
  { slug: "easy",   color: "#66bb6a" },
  { slug: "medium", color: "#ffa726" },
  { slug: "hard",   color: "#ff7043" },
  { slug: "insane", color: "#ef5350" },
] as const;

export const DIFF_SLUGS = DIFF_DEFS.map(d => d.slug);
