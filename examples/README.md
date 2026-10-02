# Examples

```
examples/
├── sample_data/                 # Images and audio used by every example
├── project/example.yaml         # Conventional project/scenes document
└── templates/
    ├── appdemo-project.yaml     # Shorthand slides document
    └── appdemo-template.yaml    # Slide-template definitions
```

Run the regular project example from the repository root:

```sh
go run . -n examples/project/example.yaml
```

Run the template example from the repository root:

```sh
go run . -n -t examples/templates/appdemo-template.yaml examples/templates/appdemo-project.yaml
```
