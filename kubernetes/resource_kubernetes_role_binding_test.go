package kubernetes

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform/helper/acctest"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
	api "k8s.io/api/rbac/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAccKubernetesRoleBinding_basic(t *testing.T) {
	var conf api.RoleBinding
	name := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	roleName := fmt.Sprintf("tf-acc-role-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:      func() { testAccPreCheck(t) },
		IDRefreshName: "kubernetes_role_binding.test",
		Providers:     testAccProviders,
		CheckDestroy:  testAccCheckKubernetesRoleBindingDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesRoleBindingConfig_basic(roleName, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesRoleBindingExists("kubernetes_role_binding.test", &conf),
					resource.TestCheckResourceAttr("kubernetes_role_binding.test", "role_ref.#", "1"),
					resource.TestCheckResourceAttr("kubernetes_role_binding.test", "role_ref.0.kind", "Role"),
					resource.TestCheckResourceAttr("kubernetes_role_binding.test", "role_ref.0.name", roleName),
					resource.TestCheckResourceAttr("kubernetes_role_binding.test", "subject.#", "1"),
					resource.TestCheckResourceAttr("kubernetes_role_binding.test", "subject.0.kind", "Group"),
				),
			},
			{
				Config: testAccKubernetesRoleBindingConfig_modified(roleName, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckKubernetesRoleBindingExists("kubernetes_role_binding.test", &conf),
					resource.TestCheckResourceAttr("kubernetes_role_binding.test", "subject.#", "2"),
					resource.TestCheckResourceAttr("kubernetes_role_binding.test", "subject.0.kind", "Group"),
					resource.TestCheckResourceAttr("kubernetes_role_binding.test", "subject.1.kind", "User"),
				),
			},
		},
	})
}

func TestAccKubernetesRoleBinding_importBasic(t *testing.T) {
	resourceName := "kubernetes_role_binding.test"
	name := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))
	roleName := fmt.Sprintf("tf-acc-role-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckKubernetesRoleBindingDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccKubernetesRoleBindingConfig_basic(roleName, name),
			},

			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"metadata.0.resource_version"},
			},
		},
	})
}

func testAccCheckKubernetesRoleBindingDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*kubernetesProvider).conn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "kubernetes_role_binding" {
			continue
		}
		namespace, name, err := idParts(rs.Primary.ID)
		if err != nil {
			return err
		}
		resp, err := conn.RbacV1().RoleBindings(namespace).Get(name, meta_v1.GetOptions{})
		if err == nil {
			if resp.Name == rs.Primary.ID {
				return fmt.Errorf("Cluster Role still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCheckKubernetesRoleBindingExists(n string, obj *api.RoleBinding) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		conn := testAccProvider.Meta().(*kubernetesProvider).conn
		namespace, name, err := idParts(rs.Primary.ID)
		if err != nil {
			return err
		}
		out, err := conn.RbacV1().RoleBindings(namespace).Get(name, meta_v1.GetOptions{})
		if err != nil {
			return err
		}

		*obj = *out
		return nil
	}
}

func testAccKubernetesRoleBindingConfig_basic(rolename, name string) string {
	return fmt.Sprintf(`
resource "kubernetes_role" "test" {
	metadata {
		name = "%s"
	}
	rule {
		api_groups = [""]
		resources  = ["pods", "pods/log"]
		verbs = ["get", "list"]
	}
}

resource "kubernetes_role_binding" "test" {
	metadata {
		name = "%s"
	}
	role_ref {
		name  = "%s"
		kind  = "Role"
	}
	subject {
		kind = "Group"
		name = "monitoring"
	}
}`, rolename, name, rolename)
}

func testAccKubernetesRoleBindingConfig_modified(rolename, name string) string {
	return fmt.Sprintf(`
resource "kubernetes_role" "test" {
	metadata {
		name = "%s"
	}
	rule {
		api_groups = [""]
		resources  = ["pods", "pods/log"]
		verbs = ["get", "list"]
	}
}

resource "kubernetes_role_binding" "test" {
	metadata {
		name = "%s"
	}
	role_ref {
		name  = "%s"
		kind  = "Role"
	}
	subject {
		kind = "Group"
		name = "monitoring"
	}
	subject {
		kind = "User"
		name = "gary"
	}
}`, rolename, name, rolename)
}
