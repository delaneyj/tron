import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { xxh32 } from "../src/xxh32";

type Xxh32Vector = {
  len: number;
  seed: string;
  result: string;
};

type VectorsFile = {
  arrays: {
    XSUM_XXH32_testdata: Xxh32Vector[];
  };
};

const findVectorsPath = (): string => {
  let dir = dirname(fileURLToPath(import.meta.url));
  for (let i = 0; i < 10; i++) {
    const candidate = join(dir, "test_vectors", "xxhash_sanity_test_vectors.json");
    if (existsSync(candidate)) {
      return candidate;
    }
    const parent = dirname(dir);
    if (parent === dir) {
      break;
    }
    dir = parent;
  }
  throw new Error("xxhash_sanity_test_vectors.json not found");
};

const loadVectors = (): Xxh32Vector[] => {
  const path = findVectorsPath();
  const raw = readFileSync(path, "utf8");
  const parsed = JSON.parse(raw) as VectorsFile;
  if (!parsed.arrays?.XSUM_XXH32_testdata?.length) {
    throw new Error("no XXH32 vectors found");
  }
  return parsed.arrays.XSUM_XXH32_testdata;
};

const generateSanityBuffer = (size: number): Uint8Array => {
  const PRIME32 = 2654435761n;
  const PRIME64 = 11400714785074694797n;
  const MASK64 = (1n << 64n) - 1n;
  let byteGen = PRIME32;
  const buffer = new Uint8Array(size);
  for (let i = 0; i < size; i++) {
    buffer[i] = Number((byteGen >> 56n) & 0xffn);
    byteGen = (byteGen * PRIME64) & MASK64;
  }
  return buffer;
};

describe("xxh32 sanity vectors", () => {
  test("matches Cyan4973/xxHash vectors", () => {
    const vectors = loadVectors();
    const maxLen = vectors.reduce((max, entry) => Math.max(max, entry.len), 0);
    const buffer = generateSanityBuffer(maxLen);

    vectors.forEach((entry) => {
      const seed = Number(BigInt(entry.seed)) >>> 0;
      const expected = Number(BigInt(entry.result)) >>> 0;
      const got = xxh32(buffer.subarray(0, entry.len), seed);
      expect(got).toBe(expected);
    });
  });
});
