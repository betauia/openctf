import {
  Globe,
  Brain,
  Lock,
  Undo2,
  CircuitBoard,
  Aperture,
  Clock,
  Star,
  Terminal,
  FingerprintPattern,
  Eye,
  Image,
} from "lucide-svelte";

export const CAT_ICON: Record<string, typeof Globe> = {
  web: Globe,
  pwn: Brain,
  crypto: Lock,
  rev: Undo2,
  hardware: CircuitBoard,
  misc: Aperture,
  afk: Clock,
  beginner: Star,
  coding: Terminal,
  forensics: FingerprintPattern,
  osint: Eye,
  stego: Image,
};
