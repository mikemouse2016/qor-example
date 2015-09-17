// +build ignore

package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jinzhu/configor"
	"github.com/manveru/faker"
	"github.com/qor/qor-example/app/models"
	"github.com/qor/qor-example/db"
	"github.com/qor/slug"
)

var fake *faker.Faker

var Tables = []interface{}{
	&models.User{}, &models.Address{},
	&models.Category{}, &models.Color{}, &models.Size{},
	&models.Product{}, &models.ColorVariation{}, &models.ColorVariationImage{}, &models.SizeVariation{},
	&models.Store{},
	&models.Order{}, &models.OrderItem{},
}

var Seeds = struct {
	Categories []struct {
		Name string
	}
	Colors []struct {
		Name string
		Code string
	}
	Sizes []struct {
		Name string
		Code string
	}
	Products []struct {
		CategoryName    string
		Name            string
		NameWithSlug    string
		Code            string
		Price           float32
		Description     string
		MadeCountry     string
		ColorVariations []struct {
			ColorName string
			Images    []struct {
				URL string
			}
		}
		SizeVariations []struct {
			SizeName string
		}
	}
	Stores []struct {
		Name      string
		Phone     string
		Email     string
		Country   string
		Zip       string
		City      string
		Region    string
		Address   string
		Latitude  float64
		Longitude float64
	}
}{}

func main() {
	fake, _ = faker.New("en")
	fake.Rand = rand.New(rand.NewSource(42))
	rand.Seed(time.Now().UnixNano())

	filepaths, _ := filepath.Glob("db/seeds/data/*.yml")
	if err := configor.Load(&Seeds, filepaths...); err != nil {
		panic(err)
	}

	truncateTables()
	createRecords()
}

func truncateTables() {
	for _, table := range Tables {
		if err := db.DB.DropTableIfExists(table).Error; err != nil {
			panic(err)
		}
		if err := db.Publish.DraftDB().DropTableIfExists(table).Error; err != nil {
			panic(err)
		}
		db.DB.AutoMigrate(table)
		db.Publish.AutoMigrate(table)
	}
}

func createRecords() {
	fmt.Println("Start create sample data...")

	createUsers()
	fmt.Println("--> Created users.")
	createAddresses()
	fmt.Println("--> Created addresses.")

	createCategories()
	fmt.Println("--> Created categories.")
	createColors()
	fmt.Println("--> Created colors.")
	createSizes()
	fmt.Println("--> Created sizes.")
	createProducts()
	fmt.Println("--> Created products.")
	createStores()
	fmt.Println("--> Created stores.")

	createOrders()
	fmt.Println("--> Created orders.")

	fmt.Println("--> Done!")
}

func createUsers() {
	for i := 0; i < 500; i++ {
		user := models.User{}
		user.Email = fake.Email()
		user.Name = fake.Name()
		user.Gender = []string{"Female", "Male"}[i%2]
		if err := db.DB.Create(&user).Error; err != nil {
			log.Fatalf("create user (%v) failure, got err %v", user, err)
		}

		user.CreatedAt = randTime()
		if err := db.DB.Save(&user).Error; err != nil {
			log.Fatalf("Save user (%v) failure, got err %v", user, err)
		}
	}
}

func createAddresses() {
	var users []models.User
	if err := db.DB.Find(&users).Error; err != nil {
		log.Fatalf("query users (%v) failure, got err %v", users, err)
	}

	for _, user := range users {
		address := models.Address{}
		address.UserID = user.ID
		address.ContactName = user.Name
		address.Phone = fake.PhoneNumber()
		address.City = fake.City()
		address.Address1 = fmt.Sprintf("%s, %s, %s", fake.StreetAddress(), address.City, fake.PostCode())
		if err := db.DB.Create(&address).Error; err != nil {
			log.Fatalf("create address (%v) failure, got err %v", address, err)
		}
	}
}

func createCategories() {
	for _, c := range Seeds.Categories {
		category := models.Category{}
		category.Name = c.Name
		if err := db.DB.Create(&category).Error; err != nil {
			log.Fatalf("create category (%v) failure, got err %v", category, err)
		}
	}
}

func createColors() {
	for _, c := range Seeds.Colors {
		color := models.Color{}
		color.Name = c.Name
		color.Code = c.Code
		if err := db.DB.Create(&color).Error; err != nil {
			log.Fatalf("create color (%v) failure, got err %v", color, err)
		}
	}
}

func createSizes() {
	for _, s := range Seeds.Sizes {
		size := models.Size{}
		size.Name = s.Name
		size.Code = s.Code
		if err := db.DB.Create(&size).Error; err != nil {
			log.Fatalf("create size (%v) failure, got err %v", size, err)
		}
	}
}

func createProducts() {
	for _, p := range Seeds.Products {
		category := findCategoryByName(p.CategoryName)

		product := models.Product{}
		product.CategoryID = category.ID
		product.Name = p.Name
		product.NameWithSlug = slug.Slug{p.NameWithSlug}
		product.Code = p.Code
		product.Price = p.Price
		product.Description = p.Description
		product.MadeCountry = p.MadeCountry

		if err := db.DB.Create(&product).Error; err != nil {
			log.Fatalf("create product (%v) failure, got err %v", product, err)
		}

		for _, cv := range p.ColorVariations {
			color := findColorByName(cv.ColorName)

			colorVariation := models.ColorVariation{}
			colorVariation.ProductID = product.ID
			colorVariation.ColorID = color.ID
			if err := db.DB.Create(&colorVariation).Error; err != nil {
				log.Fatalf("create color_variation (%v) failure, got err %v", colorVariation, err)
			}

			for _, i := range cv.Images {
				image := models.ColorVariationImage{}
				if file, err := openFileByURL(i.URL); err != nil {
					fmt.Printf("open file (%q) failure, got err %v", i.URL, err)
				} else {
					defer file.Close()
					image.Image.Scan(file)
				}
				image.ColorVariationID = colorVariation.ID
				if err := db.DB.Create(&image).Error; err != nil {
					log.Fatalf("create color_variation_image (%v) failure, got err %v", image, err)
				}
			}

			for _, sv := range p.SizeVariations {
				size := findSizeByName(sv.SizeName)

				sizeVariation := models.SizeVariation{}
				sizeVariation.ColorVariationID = colorVariation.ID
				sizeVariation.SizeID = size.ID
				sizeVariation.AvailableQuantity = 20
				if err := db.DB.Create(&sizeVariation).Error; err != nil {
					log.Fatalf("create size_variation (%v) failure, got err %v", sizeVariation, err)
				}
			}
		}
	}
}

func createStores() {
	for _, s := range Seeds.Stores {
		store := models.Store{}
		store.Name = s.Name
		store.Phone = s.Phone
		store.Email = s.Email
		store.Country = s.Country
		store.City = s.City
		store.Region = s.Region
		store.Address = s.Address
		store.Zip = s.Zip
		store.Latitude = s.Latitude
		store.Longitude = s.Longitude
		if err := db.DB.Create(&store).Error; err != nil {
			log.Fatalf("create store (%v) failure, got err %v", store, err)
		}
	}
}

func createOrders() {
	var users []models.User
	if err := db.DB.Limit(480).Preload("Addresses").Find(&users).Error; err != nil {
		log.Fatalf("query users (%v) failure, got err %v", users, err)
	}

	var sizeVariations []models.SizeVariation
	if err := db.DB.Find(&sizeVariations).Error; err != nil {
		log.Fatalf("query sizeVariations (%v) failure, got err %v", sizeVariations, err)
	}
	var sizeVariationsCount = len(sizeVariations)

	for i, user := range users {
		order := models.Order{}
		order.UserID = user.ID
		order.ShippingAddressID = user.Addresses[0].ID
		order.BillingAddressID = user.Addresses[0].ID
		if err := db.DB.Create(&order).Error; err != nil {
			log.Fatalf("create order (%v) failure, got err %v", order, err)
		}

		sizeVariation := sizeVariations[i%sizeVariationsCount]
		product := findProductByColorVariationID(sizeVariation.ColorVariationID)
		quantity := []uint{1, 2, 3, 4, 5}[i%5]
		discountRate := []uint{0, 5, 10, 15, 20, 25}[i%6]

		orderItem := models.OrderItem{}
		orderItem.OrderID = order.ID
		orderItem.SizeVariationID = sizeVariation.ID
		orderItem.Quantity = quantity
		orderItem.Price = product.Price
		orderItem.DiscountRate = discountRate
		if err := db.DB.Create(&orderItem).Error; err != nil {
			log.Fatalf("create orderItem (%v) failure, got err %v", orderItem, err)
		}

		order.CreatedAt = user.CreatedAt.Add(time.Duration(rand.Intn(24)) * time.Hour)
		if err := db.DB.Save(&order).Error; err != nil {
			log.Fatalf("Save order (%v) failure, got err %v", order, err)
		}
	}
}

func findCategoryByName(name string) *models.Category {
	category := &models.Category{}
	if err := db.DB.Where(&models.Category{Name: name}).First(category).Error; err != nil {
		log.Fatalf("can't find category with name = %q, got err %v", name, err)
	}
	return category
}

func findColorByName(name string) *models.Color {
	color := &models.Color{}
	if err := db.DB.Where(&models.Color{Name: name}).First(color).Error; err != nil {
		log.Fatalf("can't find color with name = %q, got err %v", name, err)
	}
	return color
}

func findSizeByName(name string) *models.Size {
	size := &models.Size{}
	if err := db.DB.Where(&models.Size{Name: name}).First(size).Error; err != nil {
		log.Fatalf("can't find size with name = %q, got err %v", name, err)
	}
	return size
}

func findProductByColorVariationID(colorVariationID uint) *models.Product {
	colorVariation := models.ColorVariation{}
	product := models.Product{}

	if err := db.DB.Find(&colorVariation, colorVariationID).Error; err != nil {
		log.Fatalf("query colorVariation (%v) failure, got err %v", colorVariation, err)
		return &product
	}
	if err := db.DB.Find(&product, colorVariation.ProductID).Error; err != nil {
		log.Fatalf("query product (%v) failure, got err %v", product, err)
		return &product
	}
	return &product
}

func randTime() time.Time {
	return time.Now().Add((time.Duration(-rand.Intn(7*24)) * time.Hour))
}

func openFileByURL(rawURL string) (*os.File, error) {
	if fileURL, err := url.Parse(rawURL); err != nil {
		return nil, err
	} else {
		path := fileURL.Path
		segments := strings.Split(path, "/")
		fileName := segments[len(segments)-1]

		basePath, _ := filepath.Abs(".")
		filePath := fmt.Sprintf("%s/tmp/%s", basePath, fileName)

		if _, err := os.Stat(filePath); err == nil {
			return os.Open(filePath)
		}

		file, err := os.Create(filePath)
		if err != nil {
			return file, err
		}

		check := http.Client{
			CheckRedirect: func(r *http.Request, via []*http.Request) error {
				r.URL.Opaque = r.URL.Path
				return nil
			},
		}
		resp, err := check.Get(rawURL) // add a filter to check redirect
		if err != nil {
			return file, err
		}
		defer resp.Body.Close()
		fmt.Printf("----> Downloaded %v\n", rawURL)

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return file, err
		}
		return file, nil
	}
}