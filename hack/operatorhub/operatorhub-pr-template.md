Thanks submitting your Operator. Please check below list before you create your Pull Request.

### New Submissions

* [x] Are you familiar with our [contribution guidelines](https://github.com/operator-framework/community-operators/blob/master/docs/contributing-via-pr.md)?
* [x] Have you [packaged and deployed](https://github.com/operator-framework/community-operators/blob/master/docs/testing-operators.md) your Operator for Operator Framework?
* [x] Have you tested your Operator with all Custom Resource Definitions?
* [x] Have you tested your Operator in all supported [installation modes](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/doc/design/building-your-csv.md#operator-metadata)?
* [x] Have you considered whether you want to use [semantic versioning order](https://github.com/operator-framework/community-operators/blob/master/docs/operator-ci-yaml.md#semver-mode)?
* [x] Is your submission [signed](https://github.com/operator-framework/community-operators/blob/master/docs/contributing-prerequisites.md#sign-your-work)?
* [ ] Is operator [icon](https://github.com/operator-framework/community-operators/blob/master/docs/packaging-operator.md#operator-icon) set?

### Updates to existing Operators

* [x] Did you create a `ci.yaml` file according to the [update instructions](https://github.com/operator-framework/community-operators/blob/master/docs/operator-ci-yaml.md)?
* [x] Is your new CSV pointing to the previous version with the `replaces` property if you chose `replaces-mode` via the `updateGraph` property in `ci.yaml`?
* [x] Is your new CSV referenced in the [appropriate channel](https://github.com/operator-framework/community-operators/blob/master/docs/packaging-operator.md#channels) defined in the `package.yaml` or `annotations.yaml` ?
* [x] Have you tested an update to your Operator when deployed via OLM?
* [x] Is your submission [signed](https://github.com/operator-framework/community-operators/blob/master/docs/contributing-prerequisites.md#sign-your-work)?

### Your submission should not

* [x] Modify more than one operator
* [x] Modify an Operator you don't own
* [x] Rename an operator - please remove and add with a different name instead
* [x] Modify any files outside the above-mentioned folders
* [x] Contain more than one commit. **Please squash your commits.**

### Operator Description must contain (in order)

1. [x] Description about the managed Application and where to find more information
2. [x] Features and capabilities of your Operator and how to use it
3. [x] Any manual steps about potential pre-requisites for using your Operator

### Operator Metadata should contain

* [x] Human-readable name and 1-liner description about your Operator
* [x] Valid [category name](https://github.com/operator-framework/community-operators/blob/master/docs/packaging-operator.md#categories)<sup>1</sup>
* [x] One of the pre-defined [capability levels](https://github.com/operator-framework/operator-courier/blob/4d1a25d2c8d52f7de6297ec18d8afd6521236aa2/operatorcourier/validate.py#L556)<sup>2</sup>
* [x] Links to the maintainer, source code and documentation
* [x] Example templates for all Custom Resource Definitions intended to be used
* [x] A quadratic logo

Remember that you can preview your CSV [here](https://operatorhub.io/preview).

--

<sup>1</sup> If you feel your Operator does not fit any of the pre-defined categories, file an issue against this repo and explain your need

<sup>2</sup> For more information see [here](https://sdk.operatorframework.io/docs/overview/#operator-capability-level)
