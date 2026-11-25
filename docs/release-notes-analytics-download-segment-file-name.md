# `asc analytics download --segment-id` saves to a segment-specific default file name

Changed in 7.4.0. Without `--output`, `asc analytics download` names the file
itself. With `--segment-id`, that default name now includes the segment, so two
segments of one report instance no longer share a file name.

| Invocation | Default file through 7.3.1 | Default file in 7.4.0 |
|---|---|---|
| no `--segment-id` | `analytics_report_{requestId}_{instanceId}.csv.gz` | unchanged |
| `--segment-id SEGMENT_ID` | `analytics_report_{requestId}_{instanceId}.csv.gz` | `analytics_report_{requestId}_{instanceId}_segment_{digest}.csv.gz` |
| `--output PATH` | `PATH` | unchanged |

`{digest}` is the first 32 hexadecimal characters of the SHA-256 of the segment
ID, for example `_segment_0ec07550cb4018a79ec82ca50ef105cb` for the ID
`SEGMENT_ID`. A very long instance ID is shortened so the name stays within
255 bytes. With `--decompress`, the decompressed copy has the same name
without `.gz`.

Who is affected: a script that passes `--segment-id` without `--output` and
then opens `analytics_report_{requestId}_{instanceId}.csv.gz` (or `.csv`).
That file is no longer written.

## Migration

- Read the path from the receipt on stdout: `filePath`, and `decompressedPath`
  with `--decompress`.

  ```bash
  FILE=$(asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID" --segment-id "SEGMENT_ID" --output-format json | jq -r '.filePath')
  ```

- Or choose the name with `--output`:

  ```bash
  asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID" --segment-id "SEGMENT_ID" --output "segment.csv.gz"
  ```

- To get every segment of an instance in one call, use
  `--all-segments --output-dir DIR`, which is new in 7.4.0.
