# Attribution

The persistent AVL rope in this package is adapted from MyGo's
`internal/rope` implementation, inspected at MyGo `v0.7.1`.

MyGo is Copyright its contributors and licensed under the MIT License.

The adaptation keeps immutable text versions and byte/rune/UTF-16/line
summaries while exposing only the operations Neverland's semantic engine needs.
It deliberately adds checked scalar-boundary access and engine-specific
conversion helpers rather than copying MyGo's UI buffer contract.

