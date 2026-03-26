# Domain Context

## What This Program Does

Two organisations each hold a dataset of individuals. Neither organisation is willing (or permitted) to share the raw data with the other. The goal is to compare the two datasets and answer the question: **how much do these two populations overlap?** — without either party ever seeing the other's records.

This program computes that overlap from two CSV files. Each file represents one dataset. Each row is a record. One or more columns in each row act as a key that identifies the individual that record relates to.

---

## The Four Metrics

Using the example datasets from the brief:

```
Dataset A: A B C D D E F F
Dataset B: A C C D F F F X Y
```

### 1. Count of keys in each file

The total number of rows in each file, duplicates included.

```
A: A B C D D E F F  →  8 keys
B: A C C D F F F X Y  →  9 keys
```

This is a data quality check before any analysis begins. If an organisation claims to have 10,000 customer records but the file contains 8,000 rows, the discrepancy needs to be explained before the results can be trusted.

### 2. Count of distinct keys in each file

The number of unique key values in each file — the real population size after deduplication.

```
A: A B C D E F  →  6 distinct  (D and F each appeared twice, collapsed to one)
B: A C D F X Y  →  6 distinct  (C appeared twice, F three times, collapsed)
```

This is the figure that represents "how many unique individuals does this organisation have records for".

### 3. Distinct overlap

The number of key values that appear in both files, regardless of how many times each appears.

```
Keys in A: A B C D E F
Keys in B: A C D F X Y
                ↕
In both:   A C D F  →  distinct overlap = 4
```

This is the **primary business answer**: how many unique individuals are present in both datasets.

Example use case: a retailer and a bank want to know if building a joint product makes sense. If they share 4,000 unique customers out of populations of 50,000 and 80,000, that audience size informs whether the partnership is commercially viable — without either party revealing who those 4,000 people are.

### 4. Total overlap

For every key that appears in both files, multiply the count in A by the count in B, then sum across all shared keys. This counts every record-pair match — i.e. the number of rows that would result from joining the two files on the key column.

```
Key A: 1 in A, 1 in B  →  1×1 = 1  →  (A_row1 matches B_row1)
Key C: 1 in A, 2 in B  →  1×2 = 2  →  (A_row1 matches B_row1, B_row2)
Key D: 2 in A, 1 in B  →  2×1 = 2  →  (A_row1 matches B_row1, A_row2 matches B_row1)
Key F: 2 in A, 3 in B  →  2×3 = 6  →  (A_row1 matches B_row1, B_row2, B_row3
                                         A_row2 matches B_row1, B_row2, B_row3)

Total overlap = 1 + 2 + 2 + 6 = 11
```

This is the **data quality signal**. In a clean dataset where each key identifies exactly one individual, total overlap should equal distinct overlap. If total overlap is significantly larger, it means records are duplicated — the same individual has been entered multiple times in one or both files. In a privacy-sensitive platform, inflated duplicates can skew any downstream analysis built on the intersection.

---

## Why Key Choice Matters

A key is only useful if it uniquely identifies an individual within a dataset. UDPRN (Unique Delivery Point Reference Number) is a strong key for this purpose because each UDPRN maps to exactly one UK delivery address. In a clean dataset, no two different people should share a UDPRN.

If a key has lower cardinality — for example a postcode, where many people share the same value — then total overlap inflates dramatically even without any data quality problems, because multiple people in A legitimately match multiple people in B on that key. In that case total overlap loses its meaning as a data quality signal.

This is why the key columns used for comparison must be explicitly specified by the caller (`--key-columns` flag) rather than assumed. The choice of key determines what the results mean.

---

## Composite Keys

A single column may not be sufficient to uniquely identify an individual. Two datasets might not share a common single key (one has UDPRN, the other has email addresses) but both might have a combination of columns that together identify a person. Composite keys — where multiple column values are combined into a single key string — allow richer matching strategies without changing the underlying algorithm.
