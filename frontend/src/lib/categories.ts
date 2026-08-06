import { Globe, Brain, Lock, Undo2, CircuitBoard, Aperture, Clock, Star, Terminal, Fingerprint, Eye, Image } from "@lucide/astro";

export const CAT_DEFS = [
  { slug: "web",       color: "#2979ff" },    /** Web category color **/
  { slug: "pwn",       color: "#ef5350" },    /** Pwn category color **/
  { slug: "crypto",    color: "#ab47bc" },    /** Crypto category color **/
  { slug: "rev",       color: "#ffa726" },    /** Rev category color **/
  { slug: "hardware",  color: "#ff7043" },    /** Hardware category color **/
  { slug: "misc",      color: "#66bb6a" },    /** Misc category color **/
  { slug: "afk",       color: "#78909c" },    /** AFK category color **/
  { slug: "beginner",  color: "#26c6da" },    /** Beginner category color **/
  { slug: "coding",    color: "#42a5f5" },    /** Coding category color **/
  { slug: "forensics", color: "#26a69a" },    /** Forensics category color **/
  { slug: "osint",     color: "#ec407a" },    /** OSINT category color **/
  { slug: "stego",     color: "#ce93d8" },    /** Stego category color **/
] as const;

export const DIFF_DEFS = [
  { slug: "easy",   color: "#66bb6a" },       /** Easy difficulty color **/
  { slug: "medium", color: "#ffa726" },       /** Medium difficulty color **/
  { slug: "hard",   color: "#ff7043" },       /** Hard difficulty color **/
  { slug: "insane", color: "#ef5350" },       /** Insane difficulty color **/
] as const;

export const CAT_SLUGS = CAT_DEFS.map(c => c.slug);
export const DIFF_SLUGS = DIFF_DEFS.map(d => d.slug);

const ICONS: Record<string, any> = {
  web: Globe, pwn: Brain, crypto: Lock, rev: Undo2, hardware: CircuitBoard, misc: Aperture,
  afk: Clock, beginner: Star, coding: Terminal, forensics: Fingerprint, osint: Eye, stego: Image,
};

export const CATS = CAT_DEFS.map(c => ({ ...c, Icon: ICONS[c.slug] }));
export const CAT_MAP = Object.fromEntries(CATS.map(c => [c.slug, c]));

