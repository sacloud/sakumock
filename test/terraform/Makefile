.PHONY: clean test

# Run the end-to-end Terraform integration test (requires the `terraform` binary).
test:
	go test -tags terraform -v ./...

# Remove artifacts left behind by running `terraform` / `sakumock all` manually
# here (the .gitignore'd Terraform state plus the generated .env). The automated
# test uses a temp dir, so these are never needed by it.
clean:
	rm -rf .terraform .terraform.lock.hcl *.tfstate *.tfstate.* .env
