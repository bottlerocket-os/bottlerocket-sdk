# Building the Bottlerocket SDK

If you'd like to build your own images instead of relying on the [published images](https://github.com/bottlerocket-os/bottlerocket-sdk#availability), follow these steps.

## Build the images

### Dependencies

#### System Requirements

The build process images can consume in excess of 40GB in the docker directory.

The build process is also fairly demanding on your CPU since we build all included software from scratch.

#### Linux

The build system requires certain operating system packages to be installed.

Ensure the following OS packages are installed:

##### Ubuntu

```shell
apt install build-essential
```

##### Fedora

```shell
yum install make
```

#### Docker

Bottlerocket uses [Docker](https://docs.docker.com/install/#supported-platforms) for image builds.

You'll need to have Docker installed and running, with your user account added to the `docker` group.
Docker's [post-installation steps for Linux](https://docs.docker.com/install/linux/linux-postinstall/) will walk you through that.

> Note: If you're on a newer Linux distribution using the unified cgroup hierarchy with cgroups v2, you may need to disable it to work with some versions of runc.
> You'll know this is the case if you see an error like `docker: Error response from daemon: OCI runtime create failed: this version of runc doesn't work on cgroups v2: unknown.`
> Set the kernel parameter `systemd.unified_cgroup_hierarchy=0` in your boot configuration (e.g. GRUB) and reboot.

### Build process

To build the SDK, run:

```shell
make
```

This will create an image that works for both **x86_64** and **aarch64** targets.

## Use your images

To use your custom built SDK, you will need to modify the `Twoliter.toml` in the top-level of your Bottlerocket OS fork or out-of-tree build.

```
[sdk]
registry = "my-custom-registry"
repo = "my-custom-repo"
tag = "my-tag"
```

Replace `my-custom-registry`, `my-custom-repo`, and `my-tag` to match what you have built/published.

From there, you will be able to [build Bottlerocket](https://github.com/bottlerocket-os/bottlerocket/blob/develop/BUILDING.md).

## Publish your images

If you'd like to build Bottlerocket on multiple host architectures, we recommend that you publish multi-arch manifests to each of the repositories you created.
See the Amazon ECR documentation for [pushing a multi-architecture image](https://docs.aws.amazon.com/AmazonECR/latest/userguide/docker-push-multi-architecture-image.html).

For easy publishing, we've added a publish target to the Makefile as well as a script `publish-sdk`.

To publish via the `publish-sdk` script, run:

```shell
./publish-sdk \
  --registry="my-custom-registry" \
  --repository="my-custom-repo" \
  --tag="my-tag" \
  --short-sha=0123abcd
```

or:

```shell
make publish REGISTRY=my-custom-registry REPOSITORY=my-custom-repo
```

to have the commit short SHA-1 hash and the tag derived from the state of the local Git repository.
