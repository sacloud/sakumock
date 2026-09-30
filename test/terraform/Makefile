.PHONY: clean test

# Run the end-to-end Terraform integration test (requires the `terraform` binary).
test:
	go test -tags terraform -v ./...

# Remove artifacts left behind by running `terraform` / `sakumock all` here
# (the .gitignore'd Terraform state plus the generated .env). The committed
# .terraform.lock.hcl is kept.
clean:
	rm -rf .terraform *.tfstate *.tfstate.* .env
